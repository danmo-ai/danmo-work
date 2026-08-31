#!/usr/bin/env python3
"""Tests for novel_gate.py (stdlib unittest)."""
from __future__ import annotations

import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import novel_gate as ng  # noqa: E402

LEDGER = """# Continuity ledger
## Public facts
| Kind | Fact | First seen | Notes |
|------|------|------------|-------|
| shown_fact | 主角在城东客栈醒来 | ch001 | |

## Tracking
### Cursor
- last_committed_ch: 0

## Open loops
| ID | Type | Summary | Planted | Status |
|----|------|---------|---------|--------|
| FS-001 | FS | 失踪信 | 1 | open |

## Chapter summaries
"""

TREE = {
    "novel/demo/novel-state.yaml": """book_id: demo
title: Demo
stage: writing
last_committed_ch: 0
qc_profile: male_power
gates:
  knowledge: pass
  asset: pass
  qc: unknown
blockers: []
""",
    "novel/demo/book-bible.md": "# bible\n",
    "novel/demo/canon/world.md": "# world\n",
    "novel/demo/canon/author-lore.md": "# author lore\n终局: 宿敌真身 v5\n",
    "novel/demo/canon/cast/.gitkeep": "",
    "novel/demo/outline/volumes/v01.md": """# Volume
### 剧情单元 U1
- 单元ID：`v01-U1`
- 章范围：ch1-ch5
- 单元节拍（章功能分配）：
  - ch1 建立期待：开局羞辱
  - ch2-ch3 尝试：反证身份
  - ch4 切断：当众打脸
  - ch5 兑现：留下失踪信
- 单元功能（本段必须完成）：开局立冲突
- 主角局部目标：活下来并反证身份
- 因果入口：开卷切口
- 核心阻碍：当众羞辱
- 关键选择：是否公开反证
- 主爽点形态：打脸
- 兑现归属：主角声望可见回升
- 禁止提前释放：宿敌真身
- 下一单元钩子：失踪信
- 终局边界：宿敌真身
""",
    "novel/demo/continuity/ledger.md": LEDGER,
    "novel/demo/reviews/.gitkeep": "",
    "novel/demo/chapters/ch001-contract.yaml": """chapter: 1
unit_id: v01-U1
title_working: 客栈
purpose: 主角被当众羞辱后反证身份
beats: ["羞辱", "反证", "留下失踪信"]
pleasure_point: 当众打脸
hook:
  type: 未兑现承诺
  out: 明日午时当众验骨
reader_debt: []
status: accepted
word_target: 3000
""",
    "novel/demo/chapters/ch001.md": "客栈里有人笑他。他亮出腰牌，对面的人脸色变了。门外有人递来一封失踪信。明日午时当众验骨。\n",
}


def write_tree(root: Path, files: dict[str, str]) -> None:
    for rel, body in files.items():
        p = root / rel
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(body, encoding="utf-8")


class GateTests(unittest.TestCase):
    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.root = Path(self._tmp.name)
        write_tree(self.root, TREE)

    def tearDown(self) -> None:
        self._tmp.cleanup()

    def test_preflight_pass(self):
        rep = ng.run(str(self.root), "demo", "preflight", 1)
        self.assertEqual(rep.verdict, "PASS", rep.format())

    def test_doctor_pass(self):
        rep = ng.run(str(self.root), "demo", "doctor", 0)
        self.assertEqual(rep.verdict, "PASS", rep.format())

    def test_doctor_no_glossary_required(self):
        # glossary.md is optional; tree already has none
        rep = ng.run(str(self.root), "demo", "doctor", 0)
        self.assertEqual(rep.verdict, "PASS", rep.format())
        self.assertFalse(any("glossary" in f["message"] for f in rep.findings))

    def test_preflight_empty_unit(self):
        p = self.root / "novel/demo/chapters/ch001-contract.yaml"
        p.write_text(p.read_text(encoding="utf-8").replace("unit_id: v01-U1", 'unit_id: ""'), encoding="utf-8")
        rep = ng.run(str(self.root), "demo", "preflight", 1)
        self.assertEqual(rep.verdict, "FAIL", rep.format())

    def test_preflight_unknown_hook(self):
        p = self.root / "novel/demo/chapters/ch001-contract.yaml"
        p.write_text(p.read_text(encoding="utf-8").replace("type: 未兑现承诺", "type: 悬念"), encoding="utf-8")
        rep = ng.run(str(self.root), "demo", "preflight", 1)
        self.assertEqual(rep.verdict, "FAIL", rep.format())

    def test_precommit_deslop(self):
        p = self.root / "novel/demo/chapters/ch001.md"
        p.write_text(
            "他目光深邃，不禁深吸一口气。这不是失败，而是命运的安排。瞳孔微缩。或许，这只是个开始……\n",
            encoding="utf-8",
        )
        rep = ng.run(str(self.root), "demo", "precommit", 1)
        self.assertEqual(rep.verdict, "FAIL", rep.format())
        self.assertTrue(any(f["check"] == "deslop" and f["severity"] == "blocking" for f in rep.findings), rep.format())

    def test_postcommit(self):
        rep = ng.run(str(self.root), "demo", "postcommit", 1)
        self.assertEqual(rep.verdict, "FAIL")
        p = self.root / "novel/demo/chapters/ch001-contract.yaml"
        p.write_text(p.read_text(encoding="utf-8").replace("status: accepted", "status: reviewed"), encoding="utf-8")
        ledger = self.root / "novel/demo/continuity/ledger.md"
        ledger.write_text(LEDGER + "## ch001 客栈\n- 事件: 打脸\n", encoding="utf-8")
        (self.root / "novel/demo/novel-state.yaml").write_text(
            "book_id: demo\nstage: writing\nlast_committed_ch: 1\nqc_profile: male_power\n",
            encoding="utf-8",
        )
        rep = ng.run(str(self.root), "demo", "postcommit", 1)
        self.assertEqual(rep.verdict, "PASS", rep.format())

    def test_legacy_continuity_accepted(self):
        book = self.root / "novel/demo"
        (book / "continuity/ledger.md").unlink()
        (book / "continuity/public-lore.md").write_text("# public\n", encoding="utf-8")
        (book / "continuity/tracking.md").write_text("# tracking\n", encoding="utf-8")
        (book / "continuity/chapter_summaries.md").write_text("## ch001 客栈\n- 事件: x\n", encoding="utf-8")
        (book / "continuity/foreshadow-tracker.md").write_text(
            "| ID | Summary | Status |\n|----|---------|--------|\n| FS-1 | x | open |\n",
            encoding="utf-8",
        )
        rep = ng.run(str(self.root), "demo", "doctor", 0)
        self.assertEqual(rep.verdict, "PASS", rep.format())
        rep = ng.run(str(self.root), "demo", "preflight", 1)
        self.assertEqual(rep.verdict, "PASS", rep.format())
        p = book / "chapters/ch001-contract.yaml"
        p.write_text(p.read_text(encoding="utf-8").replace("status: accepted", "status: reviewed"), encoding="utf-8")
        (book / "novel-state.yaml").write_text(
            "book_id: demo\nstage: writing\nlast_committed_ch: 1\nqc_profile: male_power\n",
            encoding="utf-8",
        )
        rep = ng.run(str(self.root), "demo", "postcommit", 1)
        self.assertEqual(rep.verdict, "PASS", rep.format())

    def test_doctor_orphan_prose(self):
        (self.root / "novel/demo/chapters/ch002.md").write_text("x\n", encoding="utf-8")
        rep = ng.run(str(self.root), "demo", "doctor", 0)
        self.assertEqual(rep.verdict, "FAIL", rep.format())

    def test_resolve_direct_root(self):
        book, st = ng.resolve_book(str(self.root / "novel/demo"), "")
        self.assertEqual(st.get("book_id"), "demo")
        self.assertEqual(book.name, "demo")

    def test_unknown_action(self):
        with self.assertRaises(ValueError):
            ng.run(str(self.root), "", "write", 1)

    def test_main_exit_codes(self):
        rc = ng.main(["--workdir", str(self.root), "--book-id", "demo", "--action", "doctor"])
        self.assertEqual(rc, 0)
        rc = ng.main(["--workdir", str(self.root), "--book-id", "demo", "--action", "preflight"])
        self.assertEqual(rc, 2)


if __name__ == "__main__":
    unittest.main()
