"""Audit the restored course scope without connecting to a player database."""
import argparse
import hashlib
import json
from collections import Counter
from pathlib import Path


def audit(root):
    names = ("plan", "node", "round", "resource", "attr", "benefit", "benefit_list")
    tables, hashes = {}, {}
    for name in names:
        path = root / f"child2_{name}.json"
        raw = path.read_bytes()
        tables[name] = json.loads(raw)
        hashes[path.name] = hashlib.sha256(raw).hexdigest()
    plans, nodes = tables["plan"], tables["node"]
    courses = []
    for plan in sorted(plans.values(), key=lambda row: row["id"]):
        path, seen, node_id = [], set(), plan["result_node"]
        while len(path) < 256:
            assert node_id and node_id not in seen, (plan["id"], "cycle/missing")
            seen.add(node_id)
            node = nodes[str(node_id)]
            assert node["next_type"] == 1
            successor = int(node["next"] or 0)
            path.append(node_id)
            if node["type"] == 102 and node["drop_type_client"] == 1 and successor == 0:
                break
            assert node["type"] == 1 and node["drop_type_client"] == 0 and successor
            node_id = successor
        else:
            raise AssertionError((plan["id"], "step limit"))
        owners = set()
        for field in ("cost", "result_display"):
            for kind, item, amount in plan[field]:
                assert kind in (1, 2) and amount >= 0
                table = "attr" if kind == 1 else "resource"
                owners.add(tables[table][str(item)]["character"])
        assert len(owners) == 1
        courses.append({"id": plan["id"], "owner": owners.pop(), "group": plan["group_id"],
                        "level": plan["level"], "nodes": path})
    groups = {}
    for course in courses:
        groups.setdefault(course["group"], []).append(course)
    for group in groups.values():
        assert sorted(row["level"] for row in group) == list(range(1, len(group) + 1))
    modes = Counter()
    for row in tables["round"].values():
        assert row["plan_num"] == 5 and len(set(row["plan_group"])) == len(row["plan_group"])
        for group in row["plan_group"]:
            assert all(course["owner"] == row["character"] for course in groups[group])
        modes[f'role{row["character"]}/hard{row["is_hard_mode"]}/type{row["round_type"]}'] += 1
    extra = []
    for buff in tables["benefit_list"].values():
        for id_ in buff["show_content"]:
            benefit = tables["benefit"][str(id_)]
            if any(isinstance(effect, list) and effect[0] == 7 for effect in benefit["effect"]):
                extra.append(buff["id"])
    return {"scope": "restored 9.7.395 course configurations", "sha256": hashes,
            "course_count": len(courses), "chain_lengths": dict(Counter(len(row["nodes"]) for row in courses)),
            "round_count": sum(modes.values()), "modes": dict(sorted(modes.items())),
            "owned_extra_plan_buffs": sorted(set(extra)), "courses": courses}


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("config_dir", type=Path)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    result = json.dumps(audit(args.config_dir), ensure_ascii=False, indent=2) + "\n"
    if args.output:
        args.output.write_text(result, encoding="utf-8")
    else:
        print(result, end="")
