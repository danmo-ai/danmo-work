#!/usr/bin/env python3
"""Unit tests for fingerprint_stats.py (stdlib unittest)."""
from __future__ import annotations

import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

SCRIPTS = Path(__file__).resolve().parent
sys.path.insert(0, str(SCRIPTS))
import fingerprint_stats as fs  # noqa: E402

SAMPLE = (
    "雨停了。他推开木门，看见空荡的堂屋。\n\n"
    "「你来了。」她说。\n"
    "「我只是路过。」\n\n"
    "然而堂屋里没有第三个人。他站着，然而没有说话。"
    "接着他把伞靠在墙边——水顺着伞骨滴下来。"
)


class FingerprintStatsTests(unittest.TestCase):
    def test_analyze_stable_on_fixture(self):
        a = fs.analyze(SAMPLE)
        b = fs.analyze(SAMPLE)
        self.assertEqual(a, b)
        self.assertGreater(a["chars"], 50)
        self.assertGreater(a["sentences"], 3)
        self.assertGreater(a["avg_sentence_len"], 0)
        self.assertGreater(a["dialogue_ratio"], 0.05)
        self.assertGreater(a["em_dash_per_1k"], 0)
        self.assertGreater(a["connective_per_1k"], 0)
        self.assertGreater(a["paragraph_avg_len"], 0)

    def test_analyze_empty(self):
        got = fs.analyze("")
        self.assertEqual(got["chars"], 0)
        self.assertEqual(got["dialogue_ratio"], 0.0)

    def test_cli_json_stdout_no_prose_file(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            src = root / "excerpt.md"
            src.write_text(SAMPLE, encoding="utf-8")
            before = {p.name for p in root.iterdir()}
            proc = subprocess.run(
                [sys.executable, str(SCRIPTS / "fingerprint_stats.py"), str(src)],
                check=False,
                capture_output=True,
                text=True,
            )
            self.assertEqual(proc.returncode, 0, proc.stderr)
            data = json.loads(proc.stdout)
            self.assertIn("avg_sentence_len", data)
            self.assertEqual(data["source"], str(src))
            after = {p.name for p in root.iterdir()}
            self.assertEqual(before, after)  # no extra prose dump files

    def test_collect_directory(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            (root / "a.txt").write_text("第一句。", encoding="utf-8")
            (root / "b.md").write_text("「你好。」", encoding="utf-8")
            text = fs.collect_texts(root)
            self.assertIn("第一句", text)
            self.assertIn("你好", text)


if __name__ == "__main__":
    unittest.main()
