import json
from pathlib import Path
import tempfile
import unittest

from educate_condition_audit import audit, attach_source_report, PREFIX, CONDITION, BENEFIT, LIST


class ReachabilityEvidenceTests(unittest.TestCase):
    def snapshot(self, *, show=False, indirect=False, scalar=False, unknown=False):
        rows = [
            (CONDITION, 10010, {"id": 10010, "type": 7}),
            (CONDITION, 10012, {"id": 10012, "type": 9}),
            (BENEFIT, 24, {"id": 24, "effect": [], "condition": ["&&", [10010]]}),
            (BENEFIT, 26, {"id": 26, "effect": [], "condition": ["&&", [10012]]}),
            (BENEFIT, 1001, {"id": 1001, "effect": [24] if scalar else [[14, 24]] if indirect else [],
                             "condition": "future_condition_syntax" if unknown else []}),
            (LIST, 1001, {"id": 1001, "content": [1001], "show_content": [24] if show else [1001]}),
            # Numeric strings can be node IDs or display text, not condition refs.
            (PREFIX + "node.json", 10009, {"\ufeffid": 10009, "next_type": 1, "next": "10010"}),
            (PREFIX + "polaroid.json", 1, {"id": 1, "condition": "Displayed lock description"}),
        ]
        with tempfile.TemporaryDirectory() as folder:
            source = Path(folder) / "config.jsonl"
            source.write_text("\n".join(json.dumps({"category": category, "key": str(key), "data": data})
                                        for category, key, data in rows), encoding="utf-8")
            return audit(source)

    def test_unreferenced_samples_and_same_number_nodes(self):
        result = self.snapshot()
        self.assertTrue(result["reachability_model_closed"])
        self.assertFalse(result["type7_9"]["10010"]["reachable_in_conservative_config_model"])
        self.assertFalse(result["type7_9"]["10012"]["reachable_in_conservative_config_model"])
        self.assertEqual(result["classified_numeric_coincidences"][0]["namespace"], "fixed_successor_node_id")

    def test_display_only_and_indirect_grants_prevent_false_exclusion(self):
        for options in ({"show": True}, {"indirect": True}, {"scalar": True}):
            with self.subTest(options=options):
                result = self.snapshot(**options)
                self.assertTrue(result["reachability_model_closed"])
                self.assertTrue(result["type7_9"]["10010"]["reachable_in_conservative_config_model"])

    def test_new_expression_grammar_keeps_model_open(self):
        result = self.snapshot(unknown=True)
        self.assertFalse(result["reachability_model_closed"])
        self.assertEqual(result["expression_errors"][0]["category"], BENEFIT)
        self.assertEqual(result["expression_errors"][0]["field"], "condition")

    def test_official_source_claim_requires_the_exact_restored_input(self):
        result = self.snapshot()
        provenance = {"status": "validated", "source_kind": "official_resource_plus_local_apk_metadata",
                      "snapshot_sha256": result["source_sha256"], "resource_sha256": "synthetic-resource",
                      "metadata_sha256": "synthetic-metadata", "decoder_sha256": {}, "versions": []}
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "report.json"
            path.write_text(json.dumps(provenance), encoding="utf-8")
            attach_source_report(result, path)
            self.assertIn("official_decoded_snapshot", result["source_kind"])
            self.assertFalse(result["source_provenance"]["private_server_semantics_proved"])
            for changes in ({"snapshot_sha256": "different-input"}, {"status": "error"}):
                path.write_text(json.dumps({**provenance, **changes}), encoding="utf-8")
                with self.subTest(changes=changes), self.assertRaisesRegex(ValueError, "exact snapshot"):
                    attach_source_report(result, path)


if __name__ == "__main__":
    unittest.main()
