"""Capture the existing read-only version/hash/resource interfaces as evidence.

Uses the workspace's already verified protocol module, supplied explicitly.
Never authenticates an account, changes hosts, imports configs, or deploys.
"""
import argparse
import csv
from datetime import datetime, timezone
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import socket
import struct
import urllib.parse
import urllib.request


def decode_version(protocol, wire):
    if len(wire) < 7 or struct.unpack(">H", wire[:2])[0] + 2 != len(wire):
        raise ValueError("truncated or mismatched version frame")
    pid = struct.unpack(">H", wire[3:5])[0]
    if pid != 10801:
        raise ValueError(f"expected SC_10801, got {pid}")
    fields = protocol.parse(wire[7:])
    versions = [v.decode("utf-8") for fn, wt, v in fields if fn == 4 and wt == 2]
    cdns = [v.decode("utf-8") for fn, wt, v in fields if fn == 10 and wt == 2]
    if not versions:
        raise ValueError("version reply has no version tokens")
    return {"response_pid": pid, "versions": versions, "cdn_list": cdns,
            "response_bytes": len(wire), "response_sha256": hashlib.sha256(wire).hexdigest()}


def save_json(path, value):
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2), encoding="utf-8")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--protocol-module", required=True, type=Path)
    parser.add_argument("--out", required=True, type=Path)
    parser.add_argument("--host", default="line1-login-bili-blhx.bilibiligame.net")
    parser.add_argument("--replay", type=Path, help="Decode an existing response without network access")
    parser.add_argument("--manifest", action="store_true")
    parser.add_argument("--resource", help="Optional exact logical resource path, e.g. scripts64")
    args = parser.parse_args()
    if args.replay and (args.manifest or args.resource):
        parser.error("replay cannot download a manifest or resource")
    if args.resource and not args.manifest:
        parser.error("a resource requires its manifest")
    spec = importlib.util.spec_from_file_location("verified_version_protocol", args.protocol_module)
    protocol = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(protocol)
    args.out.mkdir(parents=True, exist_ok=False)
    report = {"utc": datetime.now(timezone.utc).isoformat(), "account_used": False,
              "protocol_module_sha256": hashlib.sha256(args.protocol_module.read_bytes()).hexdigest(),
              "source_kind": "recorded_version_replay" if args.replay else "official_version_tcp",
              "host": None if args.replay else args.host, "state": 59, "platform": "0"}
    try:
        if args.replay:
            wire = args.replay.read_bytes()
        else:
            request = protocol.frame(10800, protocol.fv(1, 59) + protocol.fb(2, "0"))
            (args.out / "request.bin").write_bytes(request)
            report["request_sha256"] = hashlib.sha256(request).hexdigest()
            with socket.create_connection((args.host, 80), 15) as sock:
                sock.sendall(request)
                header = protocol.recvn(sock, 7)
                size = struct.unpack(">H", header[:2])[0]
                if size < 5:
                    raise ValueError("invalid version frame size")
                wire = header + protocol.recvn(sock, size - 5)
        (args.out / "response.bin").write_bytes(wire)
        report.update(decode_version(protocol, wire))
        if args.manifest:
            token = next(v for v in report["versions"] if v.startswith("$azhash$"))
            cdn = report["cdn_list"][0].rstrip("/")
            url = cdn + "/android/hash/" + urllib.parse.quote(token, safe="$")
            with urllib.request.urlopen(url, timeout=30) as response:
                data = response.read(32 * 1024 * 1024 + 1)
            if len(data) > 32 * 1024 * 1024:
                raise ValueError("manifest exceeds capture limit")
            rows = list(csv.reader(io.StringIO(data.decode("utf-8-sig"))))
            (args.out / "hashes.csv").write_bytes(data)
            report["manifest"] = {"url": url, "bytes": len(data), "rows": len(rows),
                                  "sha256": hashlib.sha256(data).hexdigest()}
            if args.resource:
                matches = [row for row in rows if row and row[0] == args.resource]
                if len(matches) != 1 or len(matches[0]) != 3:
                    raise ValueError("resource requires a unique path,size,md5 row")
                name, expected_size, expected_md5 = matches[0]
                expected_size = int(expected_size)
                if expected_size < 0 or expected_size > 128 * 1024 * 1024:
                    raise ValueError("resource exceeds capture limit")
                url = cdn + "/android/resource/" + expected_md5
                with urllib.request.urlopen(url, timeout=30) as response:
                    data = response.read(expected_size + 1)
                digest = hashlib.md5(data).hexdigest()
                if len(data) != expected_size or digest != expected_md5:
                    raise ValueError("resource size or MD5 mismatch")
                # Fixed output filename; resource names never become host paths.
                (args.out / "resource.bin").write_bytes(data)
                report["resource"] = {"url": url, "logical_path": name, "bytes": len(data),
                                      "md5": digest, "sha256": hashlib.sha256(data).hexdigest(),
                                      "header_hex": data[:16].hex()}
        report["status"] = "captured"
        save_json(args.out / "report.json", report)
        print(json.dumps(report, ensure_ascii=False))
    except Exception as exc:
        report.update(status="error", error=f"{type(exc).__name__}: {exc}")
        save_json(args.out / "report.json", report)
        raise


if __name__ == "__main__":
    main()
