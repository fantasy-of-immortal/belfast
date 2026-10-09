import struct
import unittest

from restore_official_educate import extract_textasset, normalize, validate_rows


class OfficialSnapshotQualityTests(unittest.TestCase):
    def test_scalar_only_candidate_cannot_pass_as_complete_conditions_or_effects(self):
        for name, row in (("child2_condition", {"id": 1, "type": 16}),
                          ("child2_benefit", {"id": 1, "trigger": 3}),
                          ("child2_benefit_list", {"id": 1})):
            with self.subTest(table=name), self.assertRaisesRegex(ValueError, "missing required field"):
                validate_rows(name, {"1": row}, [1])

    def test_declared_ids_and_binary_textasset_are_preserved(self):
        row = {"id": 1, "type": 16, "param": [3]}
        validate_rows("child2_condition", {"1": row}, [1])
        for index in ([1, 1], [1, 2]):
            with self.subTest(index=index), self.assertRaisesRegex(ValueError, "declared all index"):
                validate_rows("child2_condition", {"1": row}, index)
        content = bytes((0x1b, 0x4c, 0x4a, 2, 2, 0xff, 0x80, 0))
        raw = struct.pack("<i", 3) + b"cfg\0" + struct.pack("<i", len(content)) + content
        self.assertEqual(extract_textasset(raw), content)
        with self.assertRaisesRegex(ValueError, "script length"):
            extract_textasset(raw[:-1])
        self.assertEqual(normalize({"param": [3], "desc": "a\r\nb"}), {"param": [3], "desc": "a\nb"})


if __name__ == "__main__":
    unittest.main()
