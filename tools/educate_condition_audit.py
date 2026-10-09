"""Read-only, conservative condition reachability evidence for a config snapshot.

No database, account, network, or game-state writes. Input may be supplied by a
future official-asset decoder. Reports never label a guessed rule as verified.
"""
import argparse
from collections import Counter, defaultdict, deque
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path

PREFIX = "ShareCfg/child2_"
CONDITION = PREFIX + "condition.json"
BENEFIT = PREFIX + "benefit.json"
LIST = PREFIX + "benefit_list.json"
FIELDS = {"condition", "level_condition", "option_condition", "option_condition_show"}


def condition_ids(value):
    if type(value) is int:
        if value < 0:
            raise ValueError("negative condition ID")
        return {value} if value else set()
    if value == []:
        return set()
    if value == "${num}":
        # Only recognize it as a symbolic leaf for reference collection;
        # this does NOT implement its value, boolean, or multiplier semantics.
        return set()
    if (isinstance(value, list) and len(value) == 2
            and value[0] in ("&&", "||") and isinstance(value[1], list)):
        return set().union(*(condition_ids(item) for item in value[1]))
    raise ValueError("unrecognized condition expression shape")


def integers(value):
    if type(value) is int:
        yield abs(value)
    elif isinstance(value, str) and value.lstrip("-").isdigit():
        yield abs(int(value))
    elif isinstance(value, list):
        for child in value:
            yield from integers(child)
    elif isinstance(value, dict):
        for child in value.values():
            yield from integers(child)


def audit(source):
    raw = source.read_bytes()
    rows = [json.loads(line) for line in raw.decode("utf-8-sig").splitlines() if line.strip()]
    tables = defaultdict(dict)
    for row in rows:
        key = str(row["key"])
        if key in tables[row["category"]]:
            raise ValueError(f"duplicate input key {row['category']}/{key}")
        tables[row["category"]][key] = row["data"]
    conditions = {int(k): v for k, v in tables[CONDITION].items()}
    benefits = {int(k): v for k, v in tables[BENEFIT].items()}
    lists = {int(k): v for k, v in tables[LIST].items()}
    if not conditions or not benefits or not lists:
        raise ValueError("snapshot missing condition/benefit/benefit_list tables")
    refs, errors, root_conditions = defaultdict(list), [], set()
    observed_fields = set()

    def walk(row, value, path=""):
        if isinstance(value, dict):
            for key, child in value.items():
                if "condition" in key:
                    observed_fields.add(key)
                if key in FIELDS:
                    if row["category"] == PREFIX + "polaroid.json" and key == "condition":
                        # NewEducatePolaroidLayer displays this string with
                        # setText(lock/Text, config.condition); it is not an AST.
                        if not isinstance(child, str):
                            errors.append({"category": row["category"], "key": row["key"],
                                           "field": path + key, "error": "polaroid lock text must be a string"})
                        continue
                    try:
                        ids = condition_ids(child)
                    except ValueError as exc:
                        errors.append({"category": row["category"], "key": row["key"],
                                       "field": path + key, "error": str(exc)})
                        continue
                    for cid in ids:
                        refs[cid].append({"category": row["category"],
                                          "key": row["key"], "field": path + key})
                    if row["category"] != BENEFIT:
                        root_conditions.update(ids)
                else:
                    walk(row, child, path + key + ".")
        elif isinstance(value, list):
            for index, child in enumerate(value):
                walk(row, child, path + str(index) + ".")

    for row in rows:
        if row["category"].startswith(PREFIX):
            walk(row, row["data"])

    # Treat EVERY list as a root, even lists not offered by current UI/config.
    # Include display-only content; absence from "content" alone is insufficient.
    roots, missing = set(), []
    list_refs = defaultdict(list)
    for lid, listing in lists.items():
        for field in ("content", "show_content"):
            values = listing[field]
            if not isinstance(values, list) or any(type(x) is not int for x in values):
                raise ValueError(f"benefit_list/{lid}/{field}: invalid ID list")
            for bid in values:
                list_refs[bid].append({"list": lid, "field": field})
                if bid in benefits:
                    roots.add(bid)
                else:
                    missing.append({"list": lid, "field": field, "benefit": bid})

    # Effect 14, for example, can name an existing benefit rather than a list.
    # Do not invent its meaning: over-approximate ALL integer effect parameters
    # that coincide with benefit IDs as possible references, regardless of kind.
    edges, incoming = defaultdict(set), defaultdict(set)
    for bid, benefit in benefits.items():
        effects = benefit["effect"]
        if not isinstance(effects, list):
            raise ValueError(f"benefit/{bid}/effect: invalid outer list")
        for effect in effects:
            params = effect[1:] if isinstance(effect, list) else effect
            for target in integers(params):
                if target in benefits:
                    edges[bid].add(target)
                    incoming[target].add(bid)
    reached = set(roots)
    queue = deque(sorted(roots))
    while queue:
        for target in edges[queue.popleft()] - reached:
            reached.add(target)
            queue.append(target)
    reached_conditions = set(root_conditions)
    for bid in reached:
        try:
            reached_conditions.update(condition_ids(benefits[bid]["condition"]))
        except ValueError:
            # Already recorded with category/key/field by walk(); the model
            # remains open and cannot establish exclusion.
            pass

    samples = {}
    for cid, condition in conditions.items():
        if condition["type"] not in (7, 9):
            continue
        uses = refs[cid]
        bid_refs = sorted({int(ref["key"]) for ref in uses if ref["category"] == BENEFIT})
        samples[str(cid)] = {"type": condition["type"], "direct_references": uses,
                            "benefit_ids": bid_refs,
                            "list_references": {str(b): list_refs[b] for b in bid_refs},
                            "possible_effect_incoming": {str(b): sorted(incoming[b]) for b in bid_refs},
                            "reachable_in_conservative_config_model": cid in reached_conditions}

    # Check critical IDs outside recognized expressions too (e.g. next/branch).
    # Numerical coincidences remain candidates; silently discarding them would
    # turn a partial field scan into a false proof.
    critical = {int(k) for k in samples}
    mentions, classified_mentions = [], []

    def other_mentions(row, value, path=""):
        if isinstance(value, dict):
            for key, child in value.items():
                if key in FIELDS or (not path and key.lstrip("\ufeff") == "id"):
                    continue
                other_mentions(row, child, path + key + ".")
        elif isinstance(value, list):
            for index, child in enumerate(value):
                other_mentions(row, child, path + str(index) + ".")
        elif (type(value) is int or isinstance(value, str) and value.isdigit()) and int(value) in critical:
            mention = {"category": row["category"], "key": row["key"],
                       "field": path.rstrip("."), "condition_candidate": int(value)}
            namespace = None
            if row["category"] == LIST and path == "name.":
                namespace = "benefit_list_display_name"
            elif row["category"] == PREFIX + "site_display.json" and path.startswith("position."):
                namespace = "site_display_xy_coordinate"
            elif row["category"] == PREFIX + "node.json" and path == "next." and row["data"]["next_type"] == 1:
                namespace = "fixed_successor_node_id"
            if namespace:
                mention["namespace"] = namespace
                classified_mentions.append(mention)
            else:
                mentions.append(mention)

    for row in rows:
        if row["category"].startswith(PREFIX):
            other_mentions(row, row["data"])
    unknown_fields = observed_fields - FIELDS - {"condition_desc"}
    closed_model = not errors and not missing and not mentions and not unknown_fields
    return {"utc": datetime.now(timezone.utc).isoformat(),
            "source_sha256": hashlib.sha256(raw).hexdigest(),
            "source_kind": "local_config_snapshot; official extraction provenance not asserted",
            "model": "all_lists_content_and_show_content_plus_conservative_effect_integer_edges",
            "conditions": len(conditions), "benefits": len(benefits), "lists": len(lists),
            "root_benefits": len(roots), "overapprox_reached_benefits": len(reached),
            "effect_candidate_edges": sum(len(v) for v in edges.values()),
            "observed_condition_fields": sorted(observed_fields),
            "unknown_condition_fields": sorted(unknown_fields),
            "expression_errors": errors, "missing_list_benefits": missing,
            "untyped_critical_mentions": mentions,
            "classified_numeric_coincidences": classified_mentions,
            "reachability_model_closed": closed_model,
            "type7_9": samples,
            "symbolic_rule_counts": dict(sorted(Counter(c["type"] for c in conditions.values()
                                                        if c["type"] in (16, 17, 18, 19)).items())),
            "boundary": "Not a proof of private server behavior or a newer official asset corpus."}


def attach_source_report(report, path):
    raw = path.read_bytes()
    provenance = json.loads(raw.decode("utf-8-sig"))
    if (provenance.get("status") != "validated"
            or provenance.get("source_kind") != "official_resource_plus_local_apk_metadata"
            or provenance.get("snapshot_sha256") != report["source_sha256"]):
        raise ValueError("official source report does not validate this exact snapshot")
    report["source_kind"] = "official_decoded_snapshot; input hash matches validated restoration report"
    report["source_provenance"] = {
        "report_sha256": hashlib.sha256(raw).hexdigest(),
        "resource_sha256": provenance["resource_sha256"],
        "metadata_sha256": provenance["metadata_sha256"],
        "decoder_sha256": provenance["decoder_sha256"],
        "versions": provenance["versions"],
        "private_server_semantics_proved": False,
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config-jsonl", required=True, type=Path)
    parser.add_argument("--out", required=True, type=Path)
    parser.add_argument("--source-report", type=Path, help="Optional validated restoration report for this exact input")
    args = parser.parse_args()
    report = audit(args.config_jsonl)
    if args.source_report:
        attach_source_report(report, args.source_report)
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps({k: report[k] for k in ("source_sha256", "reachability_model_closed",
                                           "root_benefits", "overapprox_reached_benefits", "type7_9")},
                     ensure_ascii=False))


if __name__ == "__main__":
    main()
