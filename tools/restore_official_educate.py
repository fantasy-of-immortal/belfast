"""Restore child2 tables from a recorded official scripts32 update, without import.

Requires locally installed UnityPy/lupa and an explicit reviewed decoder directory
containing 30_www_endtoend_reproduce.py, 34_www_four_modes.py, bcdec.exe, ljdec.exe.
Decoder code is a dependency, not an upstream game-configuration data source.
"""
import argparse
import csv
import hashlib
import importlib
import json
import os
from pathlib import Path
import struct
import subprocess
import sys


REQUIRED = {
    "child2_condition": ("id", "type", "param"),
    "child2_benefit": ("id", "trigger", "condition", "effect"),
    "child2_benefit_list": ("id", "content", "show_content"),
}
DEPENDENCIES = ("30_www_endtoend_reproduce.py", "34_www_four_modes.py", "bcdec.exe", "ljdec.exe")


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def write_json(path, value):
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2), encoding="utf-8")


def validate_rows(name, data, index):
    if not isinstance(data, dict) or not data:
        raise ValueError(f"{name}: actual base table is missing")
    if len(index) != len(set(index)) or set(data) != {str(key) for key in index}:
        raise ValueError(f"{name}: restored rows differ from declared all index")
    for key, row in data.items():
        if not isinstance(row, dict) or str(row.get("id")) != key:
            raise ValueError(f"{name}/{key}: row ID mismatch")
        for field in REQUIRED.get(name, ()):
            if field not in row:
                raise ValueError(f"{name}/{key}/{field}: missing required field")


def extract_textasset(raw):
    name_size = struct.unpack_from("<i", raw, 0)[0]
    position = (4 + name_size + 3) & ~3
    if name_size < 0 or position + 4 > len(raw):
        raise ValueError("invalid TextAsset name length")
    size = struct.unpack_from("<i", raw, position)[0]
    if size < 0 or position + 4 + size > len(raw):
        raise ValueError("invalid TextAsset script length")
    # Read binary bytes directly; m_Script string decoding can destroy bytecode.
    return raw[position + 4:position + 4 + size]


def normalize(value):
    if isinstance(value, dict):
        return {key.lstrip("\ufeff"): normalize(val) for key, val in value.items()}
    if isinstance(value, list):
        return [normalize(val) for val in value]
    if isinstance(value, str):
        return value.replace("\r\n", "\n")
    return value


def restore_tables(source, destination, baseline):
    import lupa

    def convert(value, root=False):
        if lupa.lua_type(value) == "function":
            raise ValueError("unrecovered function in config table")
        if lupa.lua_type(value) != "table":
            return value
        keys = list(value.keys())
        if not root and all(type(key) in (int, float) and int(key) == key for key in keys):
            if sorted(keys) == list(range(1, len(keys) + 1)):
                return [convert(value[i]) for i in range(1, len(keys) + 1)]
        return {str(key).lstrip("\ufeff"): convert(value[key]) for key in keys}

    prior = {}
    if baseline:
        for line in baseline.read_text(encoding="utf-8-sig").splitlines():
            row = json.loads(line)
            key = (row["category"], str(row["key"]))
            if key in prior:
                raise ValueError("duplicate baseline key")
            prior[key] = normalize(row["data"])
    records, reports = [], []
    for path in sorted(source.glob("child2_*.lua")):
        text = path.read_text(encoding="utf-8-sig")
        lua = lupa.LuaRuntime(unpack_returned_tuples=True, register_eval=False, register_builtins=False)
        for field in ("os", "io", "package", "require", "debug", "python", "dofile", "loadfile"):
            lua.globals()[field] = None
        lua.execute("confNEO = {}; pg = {base = {}}")
        lua.execute(text.replace("\ufeffid", "id"))
        name = path.stem
        data = convert(lua.globals().pg.base[name], root=True)
        index = convert(lua.globals().pg[name].all)
        validate_rows(name, data, index)
        category = f"ShareCfg/{name}.json"
        previous = {key: row for (table, key), row in prior.items() if table == category}
        changed = [key for key in sorted(set(previous) & set(data))
                   if previous[key] != normalize(data[key])]
        reports.append({"table": name, "rows": len(data), "lua_sha256": sha(path),
                        "hidden_id_bom_replacements": text.count("\ufeffid"),
                        "changed_after_line_ending_normalization": changed,
                        "added": sorted(set(data) - set(previous)) if baseline else None,
                        "removed": sorted(set(previous) - set(data)) if baseline else None})
        write_json(destination / f"{name}.json", data)
        records.extend({"category": category, "key": key, "data": data[key]}
                       for key in sorted(data, key=int))
    return reports, records


def run_decoder(executable, arguments, workdir, log):
    flags = subprocess.CREATE_NO_WINDOW if os.name == "nt" else 0
    with log.open("wb") as output:
        result = subprocess.run([str(executable), *map(str, arguments)], cwd=workdir,
                                stdout=output, stderr=subprocess.STDOUT,
                                timeout=90, creationflags=flags)
    if result.returncode:
        raise ValueError(f"{executable.name} failed: {result.returncode}; see {log.name}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--capture-dir", required=True, type=Path)
    parser.add_argument("--metadata", required=True, type=Path)
    parser.add_argument("--decoder-dir", required=True, type=Path)
    parser.add_argument("--out", required=True, type=Path)
    parser.add_argument("--baseline", type=Path)
    args = parser.parse_args()
    args.out.mkdir(parents=True, exist_ok=False)
    report = {"status": "started", "player_database_written": False,
              "device_resources_written": False, "configuration_imported": False,
              "official_gameplay_semantics_proved": False}
    try:
        capture = json.loads((args.capture_dir / "report.json").read_text(encoding="utf-8"))
        if (capture.get("status") != "captured"
                or capture.get("source_kind") != "official_version_tcp"
                or capture.get("manifest", {}).get("sha256") != sha(args.capture_dir / "hashes.csv")):
            raise ValueError("capture report does not validate this recorded official manifest")
        rows = list(csv.reader((args.capture_dir / "hashes.csv").open(encoding="utf-8-sig")))
        matches = [row for row in rows if row and row[0] == "scripts32"]
        if len(matches) != 1 or len(matches[0]) != 3:
            raise ValueError("scripts32 requires a unique manifest row")
        asset = args.capture_dir / "scripts32.bin"
        raw = asset.read_bytes()
        if len(raw) != int(matches[0][1]) or hashlib.md5(raw).hexdigest() != matches[0][2]:
            raise ValueError("scripts32 size or MD5 differs from recorded manifest")
        report.update(versions=capture["versions"], resource_sha256=sha(asset),
                      manifest_sha256=sha(args.capture_dir / "hashes.csv"),
                      metadata_sha256=sha(args.metadata),
                      decoder_sha256={name: sha(args.decoder_dir / name) for name in DEPENDENCIES})
        sys.path.insert(0, str(args.decoder_dir.resolve()))
        m30 = importlib.import_module("30_www_endtoend_reproduce")
        m34 = importlib.import_module("34_www_four_modes")
        m30.MD = str(args.metadata.resolve())
        key = m30.load_key()
        length = struct.unpack_from(">I", raw, len(raw) - 4)[0]
        boundary = len(raw) - 4 - length
        if not 0 < boundary < len(raw) - 4:
            raise ValueError("invalid bundle trailer")
        decrypted = bytearray(raw[:boundary])
        functions = (m34.mode0, m34.mode1, m34.mode2, m34.mode3)
        for block in range(boundary // 8):
            offset = block * 8
            words = functions[block % 4](*struct.unpack_from("<2I", raw, offset), key, offset)
            struct.pack_into("<2I", decrypted, offset, *words)
        if decrypted[:8] != b"UnityFS\0" or struct.unpack_from(">q", decrypted, 30)[0] != boundary:
            raise ValueError("decoder candidate fails UnityFS signature/size")
        bundle = args.out / "scripts32.decrypted.ab"
        bundle.write_bytes(decrypted)
        import UnityPy
        env = UnityPy.load(str(bundle))
        objects = {obj.path_id: obj for obj in env.objects}
        bytecode, intermediate, source, jsondir = (args.out / name for name in ("bytes", "luajit", "lua", "json"))
        bytecode.mkdir()
        selected = []
        for name, pointer in sorted(env.container.items()):
            if "/sharecfg/child2_" not in name:
                continue
            pointer = pointer[0] if isinstance(pointer, (list, tuple)) else pointer
            content = extract_textasset(objects[pointer.m_PathID].get_raw_data())
            output = bytecode / Path(name).name
            if output.exists() or not content.startswith(b"\x1bLJ\x02\x02"):
                raise ValueError("duplicate entry or unsupported scripts32 bytecode encoding")
            output.write_bytes(content)
            selected.append(Path(name).name.removesuffix(".lua.bytes"))
        if not set(REQUIRED) <= set(selected):
            raise ValueError("core child2 tables absent from bundle")
        # bcdec refuses an existing output; ljdec requires an existing directory.
        run_decoder(args.decoder_dir.resolve() / "bcdec.exe", [bytecode.resolve(), intermediate.resolve()],
                    args.out.resolve(), args.out / "bcdec.log")
        source.mkdir()
        run_decoder(args.decoder_dir.resolve() / "ljdec.exe", [intermediate.resolve(), "-o", source.resolve(), "-f", "-s"],
                    args.out.resolve(), args.out / "ljdec.log")
        if {p.stem for p in source.glob("*.lua")} != set(selected):
            raise ValueError("decompiler output table set differs from selected bundle entries")
        jsondir.mkdir()
        tables, records = restore_tables(source, jsondir, args.baseline)
        snapshot = args.out / "official-child2.jsonl"
        snapshot.write_text("\n".join(json.dumps(row, ensure_ascii=False, separators=(",", ":"))
                                     for row in records) + "\n", encoding="utf-8")
        report.update(status="validated", source_kind="official_resource_plus_local_apk_metadata",
                      decrypted_bundle_sha256=sha(bundle), snapshot_sha256=sha(snapshot),
                      table_count=len(tables), row_count=len(records), tables=tables)
    except Exception as exc:
        report.update(status="error", error=f"{type(exc).__name__}: {exc}")
        write_json(args.out / "report.json", report)
        raise
    write_json(args.out / "report.json", report)
    print(json.dumps({key: value for key, value in report.items() if key != "tables"}))


if __name__ == "__main__":
    main()
