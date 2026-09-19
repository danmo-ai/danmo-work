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

### Cast snapshot（上场角色）

| 角色 | 位置 | 目标 | 伤势/资源 | 知情范围 |
|------|------|------|-----------|----------|
| 主角 | 城东客栈 | 反证身份 | 无伤 | 不知宿敌真身 |

### Cannot rewind

-

## Open loops
| ID | Type | Summary | Planted | Status |
|----|------|---------|---------|--------|
| FS-001 | FS | 失踪信 | 1 | open |

## Chapter summaries
"""

POST_SUMMARY = """## ch001 客栈
- 事件: 当众打脸并留下失踪信
- 状态变化: 主角: 被辱→声望回升
- 伏笔: FS-001 plant 失踪信
- 钩子: 明日午时当众验骨
- 下章指向: 验骨现场
"""

OUTLINE = """unit_id: v01-U1
chapter_range: [1, 1]
title_working: 客栈
word_target: 4000
status: accepted
function: 开局立冲突
entry: 开卷切口
desire: 活下来并反证身份
obstacle: 当众羞辱
choice: 是否公开反证
payoff: 主角声望可见回升
pleasure: 当众打脸
forbidden: ["宿敌真身"]
endgame_boundary: 宿敌真身
next_hook:
  type: 未兑现承诺
  out: 明日午时当众验骨
scenes:
  - id: S1
    beat: 建立期待
    chapter: 1
    where: 主角 | 夜 | 客栈
    want: 保住面子
    turn: 被当众羞辱
    must_land: ["有人笑他"]
  - id: S2
    beat: 兑现
    chapter: 1
    where: 主角 | 夜 | 客栈
    want: 反证身份
    turn: 留下失踪信
    must_land: ["亮出腰牌"]
chapters:
  - chapter: 1
    title_working: 客栈
    opens_on: S1
    ends_on: S2
    cut_hook: 明日午时当众验骨
    word_share: 4000
state_deltas: ["主角: 被辱→声望回升"]
info_control:
  reveals: []
  foreshadowing: ["FS-001: plant"]
"""

PROSE = """## 第1章 客栈

客栈里有人笑他。他亮出腰牌，对面的人脸色变了。门外有人递来一封失踪信。明日午时当众验骨。
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
- 章范围：ch1-ch1
- 单元功能（本段必须完成）：开局立冲突
""",
    "novel/demo/outline/units/v01-U1.yaml": OUTLINE,
    "novel/demo/units/v01-U1.md": PROSE,
    "novel/demo/continuity/ledger.md": LEDGER,
    "novel/demo/reviews/.gitkeep": "",
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
        rep = ng.run(str(self.root), "demo", "preflight", "v01-U1")
        self.assertEqual(rep.verdict, "PASS", rep.format())
        blob = rep.format()
        self.assertIn("### CONTEXT", blob)
        self.assertIn("接钩", blob)
        self.assertIn("单元功能", blob)
        self.assertIn("场面序", blob)
        self.assertIn("章切口", blob)

    def test_doctor_pass(self):
        rep = ng.run(str(self.root), "demo", "doctor", "")
        self.assertEqual(rep.verdict, "PASS", rep.format())

    def test_preflight_empty_unit(self):
        p = self.root / "novel/demo/outline/units/v01-U1.yaml"
        p.write_text(p.read_text(encoding="utf-8").replace("unit_id: v01-U1", 'unit_id: ""'), encoding="utf-8")
        rep = ng.run(str(self.root), "demo", "preflight", "v01-U1")
        self.assertEqual(rep.verdict, "FAIL", rep.format())

    def test_preflight_unknown_hook(self):
        p = self.root / "novel/demo/outline/units/v01-U1.yaml"
        p.write_text(p.read_text(encoding="utf-8").replace("type: 未兑现承诺", "type: 悬念"), encoding="utf-8")
        rep = ng.run(str(self.root), "demo", "preflight", "v01-U1")
        self.assertEqual(rep.verdict, "FAIL", rep.format())

    def test_preflight_requires_two_scenes(self):
        p = self.root / "novel/demo/outline/units/v01-U1.yaml"
        text = p.read_text(encoding="utf-8")
        text = text.split("  - id: S2")[0]
        p.write_text(text, encoding="utf-8")
        rep = ng.run(str(self.root), "demo", "preflight", "v01-U1")
        self.assertEqual(rep.verdict, "FAIL", rep.format())
        self.assertTrue(any("scenes" in f["check"] for f in rep.findings), rep.format())

    def test_precommit_deslop(self):
        p = self.root / "novel/demo/units/v01-U1.md"
        p.write_text(
            "## 第1章 客栈\n\n他目光深邃，不禁深吸一口气。这不是失败，而是命运的安排。瞳孔微缩。或许，这只是个开始……\n",
            encoding="utf-8",
        )
        rep = ng.run(str(self.root), "demo", "precommit", "v01-U1")
        self.assertEqual(rep.verdict, "FAIL", rep.format())
        self.assertTrue(any(f["check"] == "deslop" and f["severity"] == "blocking" for f in rep.findings), rep.format())

    def test_scan_deslop_line_hits(self):
        p = self.root / "novel/demo/units/v01-U1.md"
        p.write_text(
            "## 第1章 客栈\n"
            "开头干净。\n"
            "他目光深邃，不禁点头。\n"
            "这不是失败，而是命运的安排。\n",
            encoding="utf-8",
        )
        rep, hits = ng.run_with_hits(str(self.root), "demo", "scan-deslop", "v01-U1")
        self.assertEqual(rep.verdict, "FAIL", rep.format())
        blob = "\n".join(hits)
        self.assertIn("units/v01-U1.md:L3:", blob)
        self.assertIn("目光深邃", blob)
        rc = ng.main(
            ["--workdir", str(self.root), "--book-id", "demo", "--action", "scan-deslop", "--unit", "v01-U1"]
        )
        self.assertEqual(rc, 1)

    def test_precommit_missing_divider(self):
        book = self.root / "novel/demo"
        outline = (book / "outline/units/v01-U1.yaml").read_text(encoding="utf-8")
        outline = (
            outline.replace("chapter_range: [1, 1]", "chapter_range: [1, 2]")
            .replace("word_target: 4000", "word_target: 8000")
            .replace("word_share: 4000", "word_share: 4000\n  - chapter: 2\n    title_working: 上门\n    opens_on: S1\n    ends_on: S2\n    cut_hook: 门开了\n    word_share: 4000")
        )
        # two scenes on ch1 only — add two for ch2 by duplicating beat lines via extra scenes
        outline = outline.replace(
            "    must_land: [\"亮出腰牌\"]\nchapters:",
            "    must_land: [\"亮出腰牌\"]\n"
            "  - id: S3\n    beat: 尝试\n    chapter: 2\n    where: 主角 | 昼 | 门口\n    want: 进去\n    turn: 门开了\n    must_land: [\"敲门\"]\n"
            "  - id: S4\n    beat: 加压\n    chapter: 2\n    where: 主角 | 昼 | 门口\n    want: 问清\n    turn: 没人应\n    must_land: [\"没人应\"]\n"
            "chapters:",
        )
        (book / "outline/units/v01-U1.yaml").write_text(outline, encoding="utf-8")
        (book / "units/v01-U1.md").write_text(
            "## 第1章 客栈\n\n客栈里有人笑他。\n## 第2章 上门\n\n他敲了门。\n",
            encoding="utf-8",
        )
        rep = ng.run(str(self.root), "demo", "precommit", "v01-U1")
        self.assertEqual(rep.verdict, "FAIL", rep.format())
        self.assertTrue(any("missing ---" in f["message"] for f in rep.findings), rep.format())

    def test_postcommit(self):
        rep = ng.run(str(self.root), "demo", "postcommit", "v01-U1")
        self.assertEqual(rep.verdict, "FAIL")
        p = self.root / "novel/demo/outline/units/v01-U1.yaml"
        p.write_text(p.read_text(encoding="utf-8").replace("status: accepted", "status: reviewed"), encoding="utf-8")
        ledger = self.root / "novel/demo/continuity/ledger.md"
        ledger.write_text(LEDGER + POST_SUMMARY, encoding="utf-8")
        (self.root / "novel/demo/novel-state.yaml").write_text(
            "book_id: demo\nstage: writing\nlast_committed_ch: 1\nqc_profile: male_power\n",
            encoding="utf-8",
        )
        rep = ng.run(str(self.root), "demo", "postcommit", "v01-U1")
        self.assertEqual(rep.verdict, "PASS", rep.format())

    def test_doctor_blocks_legacy_chapters(self):
        book = self.root / "novel/demo"
        (book / "units/v01-U1.md").unlink()
        ch = book / "chapters"
        ch.mkdir()
        (ch / "ch001.md").write_text("旧章\n", encoding="utf-8")
        rep = ng.run(str(self.root), "demo", "doctor", "")
        self.assertEqual(rep.verdict, "FAIL", rep.format())
        self.assertTrue(any(f["check"] == "migrate" for f in rep.findings), rep.format())

    def test_main_exit_codes(self):
        rc = ng.main(["--workdir", str(self.root), "--book-id", "demo", "--action", "doctor"])
        self.assertEqual(rc, 0)
        rc = ng.main(["--workdir", str(self.root), "--book-id", "demo", "--action", "preflight"])
        self.assertEqual(rc, 2)

    def test_preflight_injects_style_fingerprint(self):
        fp = self.root / "novel/demo/canon/style-fingerprint.md"
        fp.write_text(
            "# 文风指纹\n## POV 与语域\n- 视角：有限第三人称\n## 参考章\n- 不要注入\n",
            encoding="utf-8",
        )
        blob = ng.run(str(self.root), "demo", "preflight", "v01-U1").format()
        self.assertIn("风格指纹", blob)
        self.assertIn("有限第三人称", blob)
        self.assertNotIn("不要注入", blob)

    def test_precommit_english_leak_blocks(self):
        p = self.root / "novel/demo/units/v01-U1.md"
        p.write_text("## 第1章 客栈\n\n他觉得这件事 very 离谱，简直像个 joke。\n", encoding="utf-8")
        rep = ng.run(str(self.root), "demo", "precommit", "v01-U1")
        self.assertEqual(rep.verdict, "FAIL", rep.format())
        self.assertTrue(any("英文泄漏" in f["message"] for f in rep.findings), rep.format())

    def test_split_unit_prose_ok(self):
        text = "## 第1章 夜雨\n\n正文。\n\n---\n\n## 第2章 上门\n\n续。\n"
        slices, errors = ng.split_unit_prose(text)
        self.assertEqual(errors, [])
        self.assertEqual([s["chapter"] for s in slices], [1, 2])
        self.assertIn("正文", slices[0]["body"])
        self.assertNotIn("---", slices[0]["body"])

    def test_unknown_action(self):
        with self.assertRaises(ValueError):
            ng.run(str(self.root), "demo", "write", "v01-U1")


if __name__ == "__main__":
    unittest.main()
