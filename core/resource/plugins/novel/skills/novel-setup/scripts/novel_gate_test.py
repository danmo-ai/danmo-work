#!/usr/bin/env python3
"""Tests for novel_gate.py (stdlib unittest)."""
from __future__ import annotations

import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import novel_gate as ng  # noqa: E402

FACTS = """# Continuity facts
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

# Legacy ledger body (same shape as facts) for fallback coverage.
LEDGER = FACTS.replace("# Continuity facts", "# Continuity ledger")

POST_SUMMARY = """## ch001 客栈
- 事件: 当众打脸并留下失踪信
- 状态变化: 主角: 被辱→声望回升
- 伏笔: FS-001 plant 失踪信
- 钩子: 明日午时当众验骨
- 下章指向: 验骨现场
"""

LOCKED_TERMS = """locked_until:
  v5:
    - "方子衡"
compliance:
  - "看守所内自杀"
aliases:
  "方子衡":
    - "方主任"
"""

CAST_ZHUJUE = """# 主角

`status`: canon
`role`: protagonist

## 功能

- 本职（一句话）：落魄捕快
- 退场：仍在场

## 四件套（protagonist / volume_antagonist 必填）

- 欲望（此刻要什么，可观察）：反证身份
- 伤口（为何怕失去 / 为何偏执）：被冤
- 手段（智 / 力 / 情 / 骗）：智
- 代价底线（绝不做 / 一定会做）：不杀无辜

## 知识边界

- 已知：自己是捕快
- 不知：宿敌真身
- 误以为：失踪信是伪造

## 三锚点（气质锁定）

- **视觉**：左手缠布
- **语言**：短句，口头禅「说重点」
- **行为**：压力下摸腰牌

## 语言习惯（可观察）

- 口头禅（≤2）：说重点

## 台词样例（各 ≤40 字）

- 压力下：说重点，谁的信。
- 日常：先吃饭。
- 掩饰 / 说谎：我没见过他。

## 关系

| 对方 | 类型 | 当前质态 | 最近变化点（单元级） | 下一预期节点 |
|------|------|----------|----------------------|--------------|
| lin-xue | 旧识 | 猜疑 | v01-U1 客栈重逢 | 同盟 |
"""

CAST_LINXUE = """# 林雪

`status`: canon
`role`: recurring

## 功能

- 与主角相交点（recurring 必填）：客栈掌柜
- 退场：第二卷退

## 三锚点

- **视觉**：银簪
- **行为**：擦柜台

## 语言习惯

- 口头禅（≤2）：客官慢走

## 台词样例

- 压力下：柜台后面没人。

## 关系

| 对方 | 类型 | 当前质态 | 最近变化点（单元级） | 下一预期节点 |
|------|------|----------|----------------------|--------------|
| zhu-jue | 旧识 | 信任 | v01-U1 客栈重逢 | 同盟 |
"""

VOLUME = """# Volume outline — v01 客栈

## 卷目标

一句话：反证身份。

## 单元索引（只索引，不展开）

| unit_id | 章范围 | 一句话功能 | 本单元终局边界（禁碰） | 下一单元钩子类型 |
|---------|--------|------------|------------------------|------------------|
| v01-U1 | ch1–ch1 | 开局立冲突 | 宿敌真身 | 未兑现承诺 |

## 本卷人物（stem；批准即 canon）

- zhu-jue
- lin-xue
"""

OUTLINE = """unit_id: v01-U1
chapter_range: [1, 1]
title_working: 客栈
word_target: 4000
status: accepted
on_stage: [zhu-jue, lin-xue]
pov: zhu-jue
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
genre: 玄幻
subgenre: 传统玄幻
qc_profile: male_power
artifacts:
  cast_registry: missing
gates:
  knowledge: pass
  asset: pass
  qc: unknown
blockers: []
""",
    "novel/demo/book-bible.md": "# bible\n",
    "novel/demo/canon/world.md": "# world\n",
    "novel/demo/canon/author-lore.md": "# author lore\n终局: 宿敌真身 v5\n",
    "novel/demo/canon/locked-terms.yaml": LOCKED_TERMS,
    "novel/demo/canon/cast/zhu-jue.md": CAST_ZHUJUE,
    "novel/demo/canon/cast/lin-xue.md": CAST_LINXUE,
    "novel/demo/outline/volumes/v01.md": VOLUME,
    "novel/demo/outline/units/v01-U1.yaml": OUTLINE,
    "novel/demo/units/v01-U1.md": PROSE,
    "novel/demo/continuity/facts.md": FACTS,
    "novel/demo/continuity/commits/.gitkeep": "",
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
        self.assertIn("单元卡 v01-U1", blob)
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

    def test_scan_deslop_fan_an_variants(self):
        """lieflat-style 翻案腔 extensions: 并非/不在于/与其说 (gate P0)."""
        p = self.root / "novel/demo/units/v01-U1.md"
        cases = [
            ("真正的壁垒并非技术，而是认知。", "并非"),
            ("关键不在于人手，而在于方向。", "不在于"),
            ("与其说他赢了，不如说对手让了。", "与其说"),
        ]
        for prose, needle in cases:
            with self.subTest(needle=needle):
                p.write_text(f"## 第1章 客栈\n开头干净。\n{prose}\n", encoding="utf-8")
                rep, hits = ng.run_with_hits(str(self.root), "demo", "scan-deslop", "v01-U1")
                self.assertEqual(rep.verdict, "FAIL", rep.format())
                blob = "\n".join(hits)
                self.assertIn("毒句式", blob)
                self.assertTrue(any(needle in h for h in hits), hits)

    def test_scan_deslop_shi_de_emphasis(self):
        """「是……的。」强调框架：用户基准例拦截；短对白/不是…的 不误杀。"""
        p = self.root / "novel/demo/units/v01-U1.md"
        # blocking
        p.write_text(
            "## 第1章 客栈\n开头干净。\n红旗巷的警情，是晚上十点多才到所里的。\n",
            encoding="utf-8",
        )
        rep, hits = ng.run_with_hits(str(self.root), "demo", "scan-deslop", "v01-U1")
        self.assertEqual(rep.verdict, "FAIL", rep.format())
        self.assertTrue(any("是晚上十点多才到所里的。" in h for h in hits), hits)

        # pass: natural order
        p.write_text(
            "## 第1章 客栈\n开头干净。\n红旗巷的警情，晚上十点多才到所里。\n",
            encoding="utf-8",
        )
        rep, _ = ng.run_with_hits(str(self.root), "demo", "scan-deslop", "v01-U1")
        self.assertEqual(rep.verdict, "PASS", rep.format())

        # pass: short dialogue / 不是…的 / bare 是的
        for ok in (
            "「是他干的。」老陈说。",
            "这不是失败的结局。",
            "是的。他点了点头。",
            "他是警察。",
        ):
            with self.subTest(ok=ok):
                p.write_text(f"## 第1章 客栈\n开头干净。\n{ok}\n", encoding="utf-8")
                rep, hits = ng.run_with_hits(str(self.root), "demo", "scan-deslop", "v01-U1")
                self.assertEqual(rep.verdict, "PASS", f"{ok!r} → {hits}")

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

    def _mark_reviewed(self) -> None:
        p = self.root / "novel/demo/outline/units/v01-U1.yaml"
        p.write_text(p.read_text(encoding="utf-8").replace("status: accepted", "status: reviewed"), encoding="utf-8")
        (self.root / "novel/demo/novel-state.yaml").write_text(
            "book_id: demo\nstage: writing\nlast_committed_ch: 1\ngenre: 玄幻\nqc_profile: male_power\n",
            encoding="utf-8",
        )

    def _write_summary(self, body: str = POST_SUMMARY) -> Path:
        p = self.root / "novel/demo/continuity/summaries/v01.md"
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text("# Chapter summaries — v01\n\n" + body, encoding="utf-8")
        return p

    def test_postcommit(self):
        rep = ng.run(str(self.root), "demo", "postcommit", "v01-U1")
        self.assertEqual(rep.verdict, "FAIL")
        self._mark_reviewed()
        self._write_summary()
        rep = ng.run(str(self.root), "demo", "postcommit", "v01-U1")
        self.assertEqual(rep.verdict, "PASS", rep.format())

    def test_postcommit_summary_in_facts_only_advises_migrate(self):
        """## chNNN blocks left in facts.md still count, but postcommit points at migrate."""
        self._mark_reviewed()
        facts = self.root / "novel/demo/continuity/facts.md"
        facts.write_text(FACTS + POST_SUMMARY, encoding="utf-8")
        rep = ng.run(str(self.root), "demo", "postcommit", "v01-U1")
        self.assertEqual(rep.verdict, "PASS", rep.format())
        self.assertTrue(any("migrate" in f["message"] for f in rep.findings), rep.format())

    def test_postcommit_missing_summary_file_blocks(self):
        self._mark_reviewed()
        rep = ng.run(str(self.root), "demo", "postcommit", "v01-U1")
        self.assertEqual(rep.verdict, "FAIL", rep.format())
        self.assertTrue(any("summaries/v01.md" in f["message"] for f in rep.findings), rep.format())

    def test_postcommit_legacy_ledger_fallback(self):
        """Old books with only ledger.md (as the facts source) still pass postcommit."""
        facts = self.root / "novel/demo/continuity/facts.md"
        facts.unlink()
        ledger = self.root / "novel/demo/continuity/ledger.md"
        ledger.write_text(LEDGER, encoding="utf-8")
        self._mark_reviewed()
        self._write_summary()
        rep = ng.run(str(self.root), "demo", "postcommit", "v01-U1")
        self.assertEqual(rep.verdict, "PASS", rep.format())
        self.assertEqual(ng.ledger_path(self.root / "novel/demo").name, "ledger.md")

    def test_postcommit_relation_delta_without_card_writeback_warns(self):
        self._mark_reviewed()
        self._write_summary()
        p = self.root / "novel/demo/outline/units/v01-U1.yaml"
        p.write_text(
            p.read_text(encoding="utf-8").replace(
                'state_deltas: ["主角: 被辱→声望回升"]',
                'state_deltas: ["zhu-jue: 对 lin-xue 从猜疑→信任"]',
            ),
            encoding="utf-8",
        )
        card = self.root / "novel/demo/canon/cast/zhu-jue.md"
        card.write_text(card.read_text(encoding="utf-8").replace("v01-U1 客栈重逢", "初见"), encoding="utf-8")
        facts = self.root / "novel/demo/continuity/facts.md"
        facts.write_text(FACTS.replace("| 主角 |", "| zhu-jue |"), encoding="utf-8")
        rep = ng.run(str(self.root), "demo", "postcommit", "v01-U1")
        self.assertEqual(rep.verdict, "PASS", rep.format())
        self.assertTrue(any(f["check"] == "relation" for f in rep.findings), rep.format())

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

    def test_precommit_english_whitelist_passes(self):
        p = self.root / "novel/demo/units/v01-U1.md"
        p.write_text("## 第1章 客栈\n\n他看了一眼 GPS，转身走进 KTV。OK，就这么办。\n", encoding="utf-8")
        rep = ng.run(str(self.root), "demo", "precommit", "v01-U1")
        self.assertEqual(rep.verdict, "PASS", rep.format())
        self.assertFalse(any("英文泄漏" in f["message"] for f in rep.findings), rep.format())

    def test_precommit_emdash_density_blocks(self):
        p = self.root / "novel/demo/units/v01-U1.md"
        # 6 em-dashes in ~50 runes → 120/千字 > 5
        p.write_text("## 第1章 客栈\n\n他——她——门——灯——影——风——都静了。\n", encoding="utf-8")
        rep = ng.run(str(self.root), "demo", "precommit", "v01-U1")
        self.assertEqual(rep.verdict, "FAIL", rep.format())
        self.assertTrue(any("破折号密度" in f["message"] for f in rep.findings), rep.format())

    def test_precommit_simile_over_limit_blocks(self):
        p = self.root / "novel/demo/units/v01-U1.md"
        body = "。".join(f"第{i}句像是梦" for i in range(10))
        p.write_text(f"## 第1章 客栈\n\n{body}。\n", encoding="utf-8")
        rep = ng.run(str(self.root), "demo", "precommit", "v01-U1")
        self.assertEqual(rep.verdict, "FAIL", rep.format())
        self.assertTrue(any("比喻词" in f["message"] for f in rep.findings), rep.format())

    def test_postcommit_missing_summary_keys(self):
        p = self.root / "novel/demo/outline/units/v01-U1.yaml"
        p.write_text(p.read_text(encoding="utf-8").replace("status: accepted", "status: reviewed"), encoding="utf-8")
        facts = self.root / "novel/demo/continuity/facts.md"
        facts.write_text(FACTS + "## ch001 客栈\n- 事件: 打脸\n", encoding="utf-8")
        (self.root / "novel/demo/novel-state.yaml").write_text(
            "book_id: demo\nstage: writing\nlast_committed_ch: 1\nqc_profile: male_power\n",
            encoding="utf-8",
        )
        rep = ng.run(str(self.root), "demo", "postcommit", "v01-U1")
        self.assertEqual(rep.verdict, "FAIL", rep.format())
        self.assertTrue(any("missing summary keys" in f["message"] for f in rep.findings), rep.format())

    def test_cast_snapshot_mismatch_blocks(self):
        p = self.root / "novel/demo/outline/units/v01-U1.yaml"
        p.write_text(p.read_text(encoding="utf-8").replace("status: accepted", "status: reviewed"), encoding="utf-8")
        facts = self.root / "novel/demo/continuity/facts.md"
        facts.write_text(FACTS.replace("| 主角 |", "| 路人甲 |") + POST_SUMMARY, encoding="utf-8")
        (self.root / "novel/demo/novel-state.yaml").write_text(
            "book_id: demo\nstage: writing\nlast_committed_ch: 1\nqc_profile: male_power\n",
            encoding="utf-8",
        )
        rep = ng.run(str(self.root), "demo", "postcommit", "v01-U1")
        self.assertEqual(rep.verdict, "FAIL", rep.format())
        self.assertTrue(any("Cast snapshot" in f["message"] for f in rep.findings), rep.format())

    def test_precommit_lock_terms_blocks(self):
        p = self.root / "novel/demo/units/v01-U1.md"
        p.write_text("## 第1章 客栈\n\n方子衡走进来，客栈里有人笑他。明日午时当众验骨。\n", encoding="utf-8")
        rep = ng.run(str(self.root), "demo", "precommit", "v01-U1")
        self.assertEqual(rep.verdict, "FAIL", rep.format())
        self.assertTrue(any(f["check"] == "lock_terms" for f in rep.findings), rep.format())

    def test_precommit_lock_alias_only_while_base_locked(self):
        book = self.root / "novel/demo"
        prose = book / "units/v01-U1.md"
        prose.write_text("## 第1章 客栈\n\n方主任走进来。明日午时当众验骨。\n", encoding="utf-8")
        rep = ng.run(str(self.root), "demo", "precommit", "v01-U1")
        self.assertEqual(rep.verdict, "FAIL", rep.format())
        self.assertTrue(any(f["check"] == "lock_terms" for f in rep.findings), rep.format())
        # Unlock: move book to v05 unit — aliases must not fire once base is unlocked.
        outline = (book / "outline/units/v01-U1.yaml").read_text(encoding="utf-8")
        (book / "outline/units/v05-U1.yaml").write_text(
            outline.replace("unit_id: v01-U1", "unit_id: v05-U1"), encoding="utf-8"
        )
        (book / "units/v05-U1.md").write_text(prose.read_text(encoding="utf-8"), encoding="utf-8")
        (book / "outline/volumes/v05.md").write_text(
            "# Volume\n| U1 | ch1-ch1 | 功能 | 禁 | 信息缺口 |\n- 单元ID：`v05-U1`\n",
            encoding="utf-8",
        )
        rep2 = ng.run(str(self.root), "demo", "precommit", "v05-U1")
        self.assertFalse(any(f["check"] == "lock_terms" for f in rep2.findings), rep2.format())

    def test_precommit_word_floor_blocks(self):
        p = self.root / "novel/demo/outline/units/v01-U1.yaml"
        p.write_text(p.read_text(encoding="utf-8") + "\nword_floor: 5000\n", encoding="utf-8")
        rep = ng.run(str(self.root), "demo", "precommit", "v01-U1")
        self.assertEqual(rep.verdict, "FAIL", rep.format())
        self.assertTrue(any(f["check"] == "word_floor" for f in rep.findings), rep.format())

    def test_style_fingerprint_truncated(self):
        fp = self.root / "novel/demo/canon/style-fingerprint.md"
        fp.write_text("# 文风指纹\n" + "长" * 600 + "\n", encoding="utf-8")
        brief = ng.style_fingerprint_brief(self.root / "novel/demo")
        self.assertLessEqual(len(brief), 480 + 20, f"brief too long: {len(brief)}")

    def test_doctor_invalid_qc_profile(self):
        (self.root / "novel/demo/novel-state.yaml").write_text(
            "book_id: demo\nstage: writing\nqc_profile: banana\n",
            encoding="utf-8",
        )
        rep = ng.run(str(self.root), "demo", "doctor", "")
        self.assertEqual(rep.verdict, "FAIL", rep.format())
        self.assertTrue(any("qc_profile" in f["message"] for f in rep.findings), rep.format())

    def test_doctor_invalid_subgenre(self):
        (self.root / "novel/demo/novel-state.yaml").write_text(
            "book_id: demo\nstage: writing\ngenre: 玄幻\nsubgenre: banana\n",
            encoding="utf-8",
        )
        rep = ng.run(str(self.root), "demo", "doctor", "")
        self.assertEqual(rep.verdict, "FAIL", rep.format())
        self.assertTrue(any("subgenre=" in f["message"] for f in rep.findings), rep.format())

    def _book(self) -> Path:
        return self.root / "novel/demo"

    def _edit(self, rel: str, old: str, new: str) -> None:
        p = self._book() / rel
        text = p.read_text(encoding="utf-8")
        self.assertIn(old, text, f"{rel} lacks {old!r}")
        p.write_text(text.replace(old, new), encoding="utf-8")

    def test_candidate_on_stage_blocks(self):
        self._edit("canon/cast/lin-xue.md", "`status`: canon", "`status`: candidate")
        rep = ng.run(str(self.root), "demo", "preflight", "v01-U1")
        self.assertEqual(rep.verdict, "FAIL", rep.format())
        self.assertTrue(any("candidate" in f["message"] for f in rep.findings), rep.format())

    def test_on_stage_outside_volume_cast_blocks(self):
        self._edit("outline/volumes/v01.md", "- lin-xue\n", "")
        rep = ng.run(str(self.root), "demo", "preflight", "v01-U1")
        self.assertEqual(rep.verdict, "FAIL", rep.format())
        self.assertTrue(any("本卷人物" in f["message"] for f in rep.findings), rep.format())

    def test_no_canon_protagonist_blocks_asset_gate(self):
        self._edit("canon/cast/zhu-jue.md", "`role`: protagonist", "`role`: recurring")
        rep = ng.run(str(self.root), "demo", "preflight", "v01-U1")
        self.assertEqual(rep.verdict, "FAIL", rep.format())
        self.assertTrue(any(f["check"] == "asset" for f in rep.findings), rep.format())

    def test_preflight_context_spec(self):
        """Plan §6: genre article whole, volume row, unit card, on_stage-only cast, POV 不知."""
        rep = ng.run(str(self.root), "demo", "preflight", "v01-U1")
        self.assertEqual(rep.verdict, "PASS", rep.format())
        blob = "\n".join(rep.context_lines)
        self.assertIn("题材专有文「玄幻」", blob)
        self.assertIn("# 玄幻", blob)
        self.assertNotIn("刑侦人味文风", blob.split("- genre:")[0])
        self.assertIn("卷纲索引行: v01-U1 | ch1–ch1 | 开局立冲突 | 钩子=未兑现承诺 | 终局边界=宿敌真身", blob)
        self.assertIn("单元卡 v01-U1", blob)
        self.assertIn("choice: 是否公开反证", blob)
        self.assertIn("payoff: 主角声望可见回升", blob)
        self.assertIn("三锚点: 视觉=左手缠布", blob)
        self.assertIn("台词: 说重点，谁的信。", blob)
        self.assertIn("不知（POV 不得写出）: 宿敌真身", blob)
        self.assertIn("林雪（lin-xue，recurring）", blob)
        self.assertIn("主角 → lin-xue: 旧识 / 猜疑", blob)
        # not injected: wound / arc / chronicle
        self.assertNotIn("被冤", blob)
        self.assertNotIn("伤口", blob)

    def test_preflight_crime_subgenre_adds_flavor_article(self):
        self._edit("novel-state.yaml", "genre: 玄幻", "genre: 悬疑")
        self._edit("novel-state.yaml", "subgenre: 传统玄幻", "subgenre: 刑侦探案")
        rep = ng.run(str(self.root), "demo", "preflight", "v01-U1")
        self.assertEqual(rep.verdict, "PASS", rep.format())
        blob = "\n".join(rep.context_lines)
        self.assertIn("题材专有文「悬疑」", blob)
        self.assertIn("子类专有文「刑侦人味文风」", blob)

    def test_crime_subgenre_requires_suspense_genre(self):
        self._edit("novel-state.yaml", "subgenre: 传统玄幻", "subgenre: 刑侦探案")
        rep = ng.run(str(self.root), "demo", "doctor", "")
        self.assertEqual(rep.verdict, "FAIL", rep.format())
        self.assertTrue(any("subgenre=刑侦探案 not in genre=玄幻" in f["message"] for f in rep.findings), rep.format())

    def test_migrate_drops_craft_lane(self):
        book = self._book()
        state = (book / "novel-state.yaml").read_text(encoding="utf-8")
        state = state.replace("genre: 玄幻\nsubgenre: 传统玄幻\n", "genre: 悬疑\ncraft_lane: crime-human\n")
        (book / "novel-state.yaml").write_text(state, encoding="utf-8")
        rep = ng.run(str(self.root), "demo", "migrate", "")
        self.assertEqual(rep.verdict, "PASS", rep.format())
        migrated = (book / "novel-state.yaml").read_text(encoding="utf-8")
        self.assertNotIn("craft_lane", migrated)
        self.assertIn("subgenre: 刑侦探案", migrated)
        self.assertIn("genre: 悬疑", migrated)

    def test_preflight_empty_on_stage_has_no_snapshot_rows(self):
        self._edit("outline/units/v01-U1.yaml", "on_stage: [zhu-jue, lin-xue]\npov: zhu-jue\n", "on_stage: []\n")
        rep = ng.run(str(self.root), "demo", "preflight", "v01-U1")
        self.assertEqual(rep.verdict, "PASS", rep.format())
        blob = "\n".join(rep.context_lines)
        self.assertIn("未列上场人物", blob)
        self.assertNotIn("| 主角 | 城东客栈 |", blob)
        self.assertTrue(any(f["check"] == "on_stage" for f in rep.findings), rep.format())

    def test_function_mismatch_warns_hook_type_mismatch_blocks(self):
        self._edit("outline/units/v01-U1.yaml", "function: 开局立冲突", "function: 开局，立冲突！")
        rep = ng.run(str(self.root), "demo", "preflight", "v01-U1")
        self.assertEqual(rep.verdict, "PASS", rep.format())
        self._edit("outline/units/v01-U1.yaml", "function: 开局，立冲突！", "function: 别的功能")
        rep = ng.run(str(self.root), "demo", "preflight", "v01-U1")
        self.assertEqual(rep.verdict, "PASS", rep.format())
        self.assertTrue(any(f["check"] == "function" and f["severity"] == "advisory" for f in rep.findings), rep.format())
        self._edit("outline/units/v01-U1.yaml", "type: 未兑现承诺", "type: 倒计时")
        rep = ng.run(str(self.root), "demo", "preflight", "v01-U1")
        self.assertEqual(rep.verdict, "FAIL", rep.format())
        self.assertTrue(any("next_hook.type=倒计时 != volume index" in f["message"] for f in rep.findings), rep.format())

    def test_endgame_boundary_from_volume_requires_unit_field(self):
        self._edit("outline/units/v01-U1.yaml", 'forbidden: ["宿敌真身"]', "forbidden: []")
        self._edit("outline/units/v01-U1.yaml", "endgame_boundary: 宿敌真身", 'endgame_boundary: ""')
        rep = ng.run(str(self.root), "demo", "preflight", "v01-U1")
        self.assertEqual(rep.verdict, "FAIL", rep.format())
        self.assertTrue(any(f["check"] == "endgame_boundary" for f in rep.findings), rep.format())

    # --- accept-volume / lint-units ---

    def test_accept_volume_promotes_and_seeds(self):
        book = self._book()
        self._edit("canon/cast/lin-xue.md", "`status`: canon", "`status`: candidate")
        (book / "outline/volumes/v02.md").write_text(
            "# Volume outline — v02\n\n## 单元索引\n\n"
            "| unit_id | 章范围 | 一句话功能 | 本单元终局边界（禁碰） | 下一单元钩子类型 |\n"
            "|---|---|---|---|---|\n"
            "| v02-U1 | ch2–ch4 | 进城 | 方子衡 | 信息缺口 |\n"
            "| v02-U2 | ch5–ch6 | 验骨 | — | 倒计时 |\n\n"
            "## 本卷人物（stem；批准即 canon）\n\n- zhu-jue\n- lin-xue\n",
            encoding="utf-8",
        )
        rep = ng.run(str(self.root), "demo", "accept-volume", "", "v02")
        self.assertEqual(rep.verdict, "PASS", rep.format())
        self.assertIn("`status`: canon", (book / "canon/cast/lin-xue.md").read_text(encoding="utf-8"))
        seeded = book / "outline/units/v02-U1.yaml"
        self.assertTrue(seeded.is_file(), rep.format())
        data = ng.load_unit(book, "v02-U1")[0]
        self.assertEqual(data["unit_id"], "v02-U1")
        self.assertEqual(data["chapter_range"], [2, 4])
        self.assertEqual(data["status"], "proposed")
        self.assertEqual(data["function"], "进城")
        self.assertEqual(data["next_hook"]["type"], "信息缺口")
        self.assertEqual(data["endgame_boundary"], "方子衡")
        self.assertEqual(data["word_floor"], 3 * 2000)
        self.assertTrue((book / "outline/units/v02-U2.yaml").is_file())
        # idempotent: existing YAML is kept
        seeded.write_text(seeded.read_text(encoding="utf-8").replace("status: proposed", "status: accepted"), encoding="utf-8")
        rep2 = ng.run(str(self.root), "demo", "accept-volume", "", "v02")
        self.assertEqual(rep2.verdict, "PASS", rep2.format())
        self.assertIn("status: accepted", seeded.read_text(encoding="utf-8"))

    def test_accept_volume_blocks_on_gap_and_missing_cast(self):
        book = self._book()
        (book / "outline/volumes/v02.md").write_text(
            "# v02\n\n## 单元索引\n\n| unit_id | 章范围 | 功能 | 边界 | 钩 |\n|---|---|---|---|---|\n"
            "| v02-U1 | ch2–ch3 | 进城 | | 信息缺口 |\n| v02-U2 | ch5–ch6 | 验骨 | | 倒计时 |\n",
            encoding="utf-8",
        )
        rep = ng.run(str(self.root), "demo", "accept-volume", "", "v02")
        self.assertEqual(rep.verdict, "FAIL", rep.format())
        msgs = " ".join(f["message"] for f in rep.findings)
        self.assertIn("gap/overlap", msgs)
        self.assertIn("本卷人物", msgs)
        self.assertFalse((book / "outline/units/v02-U1.yaml").exists())

    def test_lint_units_reports_per_unit(self):
        book = self._book()
        bad = OUTLINE.replace("unit_id: v01-U1", "unit_id: v01-U2").replace("on_stage: [zhu-jue, lin-xue]", "on_stage: [zhu-jue, wai-ren]")
        (book / "outline/units/v01-U2.yaml").write_text(bad, encoding="utf-8")
        self._edit(
            "outline/volumes/v01.md",
            "| v01-U1 | ch1–ch1 | 开局立冲突 | 宿敌真身 | 未兑现承诺 |\n",
            "| v01-U1 | ch1–ch1 | 开局立冲突 | 宿敌真身 | 未兑现承诺 |\n| v01-U2 | ch2–ch2 | 上门 | | 倒计时 |\n",
        )
        rep = ng.run(str(self.root), "demo", "lint-units", "", "v01")
        self.assertEqual(rep.verdict, "FAIL", rep.format())
        blob = rep.format()
        self.assertIn("### UNITS", blob)
        self.assertIn("v01-U1: PASS", blob)
        self.assertIn("v01-U2: FAIL", blob)
        self.assertIn("wai-ren", blob)
        rc = ng.main(["--workdir", str(self.root), "--book-id", "demo", "--action", "lint-units", "--volume", "v01"])
        self.assertEqual(rc, 1)

    def test_lint_units_proposed_without_scenes_is_pending(self):
        book = self._book()
        head = ng.seed_unit_yaml(
            {"unit_id": "v01-U2", "chapter_range": [2, 3], "function": "上门", "endgame_boundary": "", "next_hook_type": "倒计时"},
            {"word_floor_per_ch": 2000, "word_ceiling_per_ch": 3500},
            ["zhu-jue"],
        )
        (book / "outline/units/v01-U2.yaml").write_text(head, encoding="utf-8")
        self._edit(
            "outline/volumes/v01.md",
            "| v01-U1 | ch1–ch1 | 开局立冲突 | 宿敌真身 | 未兑现承诺 |\n",
            "| v01-U1 | ch1–ch1 | 开局立冲突 | 宿敌真身 | 未兑现承诺 |\n| v01-U2 | ch2–ch3 | 上门 | | 倒计时 |\n",
        )
        rep = ng.run(str(self.root), "demo", "lint-units", "", "v01")
        self.assertEqual(rep.verdict, "PASS", rep.format())
        self.assertIn("待细纲", rep.format())

    # --- cast-lint ---

    def test_cast_lint_back_edge(self):
        book = self._book()
        self._edit("canon/cast/lin-xue.md", "| zhu-jue | 旧识 | 信任 | v01-U1 客栈重逢 | 同盟 |\n", "")
        rep = ng.run(str(self.root), "demo", "cast-lint", "")
        self.assertEqual(rep.verdict, "FAIL", rep.format())
        self.assertTrue(any("no back row" in f["message"] for f in rep.findings), rep.format())
        self.assertIn("cast_registry: fail", (book / "novel-state.yaml").read_text(encoding="utf-8"))
        (book / "canon/cast/lin-xue.md").write_text(CAST_LINXUE, encoding="utf-8")
        rep = ng.run(str(self.root), "demo", "cast-lint", "")
        self.assertEqual(rep.verdict, "PASS", rep.format())
        self.assertIn("cast_registry: ok", (book / "novel-state.yaml").read_text(encoding="utf-8"))

    def test_cast_lint_unknown_stem_and_missing_role(self):
        self._edit("canon/cast/zhu-jue.md", "| lin-xue | 旧识 |", "| 林雪 | 旧识 |")
        self._edit("canon/cast/lin-xue.md", "`role`: recurring\n", "")
        rep = ng.run(str(self.root), "demo", "cast-lint", "")
        self.assertEqual(rep.verdict, "FAIL", rep.format())
        msgs = " ".join(f["message"] for f in rep.findings)
        self.assertIn("对方=林雪", msgs)
        self.assertIn("`role`", msgs)

    def test_parse_cast_card(self):
        card = ng.parse_cast_card(CAST_ZHUJUE, "zhu-jue")
        self.assertEqual((card.status, card.role), ("canon", "protagonist"))
        self.assertEqual(card.anchors["视觉"], "左手缠布")
        self.assertEqual(card.dialogue["压力下"], "说重点，谁的信。")
        self.assertEqual(card.unknown, "宿敌真身")
        self.assertEqual(card.relations[0]["other"], "lin-xue")
        self.assertEqual(card.missing_fields(), [])
        rec = ng.parse_cast_card(CAST_LINXUE, "lin-xue")
        self.assertEqual(rec.missing_fields(), ["欲望"])

    # --- qc-pack ---

    def test_qc_pack_single_output(self):
        p = self._book() / "units/v01-U1.md"
        p.write_text("## 第1章 客栈\n\n开头干净。\n他目光深邃，不禁点头。明日午时当众验骨。\n", encoding="utf-8")
        rep, hits = ng.run_with_hits(str(self.root), "demo", "qc-pack", "v01-U1")
        blob = rep.format()
        self.assertIn("### COUNTS", blob)
        self.assertIn("ai_vocab_count: 3", blob)
        self.assertIn("### LENGTH", blob)
        self.assertIn("expand_needed: yes", blob)
        self.assertIn("polish_needed: yes", blob)
        self.assertIn("### HITS", blob)
        self.assertIn("units/v01-U1.md:L4:", "\n".join(hits))
        self.assertEqual(rep.verdict, "FAIL", blob)

    # --- init ---

    def test_init_builds_tree(self):
        rep = ng.run(str(self.root), "newbook", "init", "", "", title="新书", genre="悬疑")
        self.assertEqual(rep.verdict, "PASS", rep.format())
        book = self.root / "novel/newbook"
        for rel in (
            "novel-state.yaml", "book-bible.md", "canon/world.md", "canon/author-lore.md",
            "canon/locked-terms.yaml", "continuity/facts.md", "canon/style-fingerprint.md",
            "outline/book-outline.md", "outline/volumes", "outline/units", "units", "continuity/summaries", "reviews",
        ):
            self.assertTrue((book / rel).exists(), rel)
        state = (book / "novel-state.yaml").read_text(encoding="utf-8")
        self.assertIn('book_id: "newbook"', state)
        self.assertIn("genre: 悬疑", state)
        self.assertIn("stage: setup", state)
        self.assertIn("新书", (book / "continuity/facts.md").read_text(encoding="utf-8"))
        rep2 = ng.run(str(self.root), "newbook", "init", "", "", title="新书")
        self.assertEqual(rep2.verdict, "PASS", rep2.format())
        self.assertIn("(kept)", rep2.format())
        rep3 = ng.run(str(self.root), "newbook", "doctor", "")
        self.assertEqual(rep3.verdict, "PASS", rep3.format())

    # --- migrate ---

    def test_migrate_old_book(self):
        book = self._book()
        # old layout: summaries in facts, no genre, no on_stage/pov, no role, no 本卷人物
        (book / "continuity/facts.md").write_text(FACTS + POST_SUMMARY, encoding="utf-8")
        (book / "novel-state.yaml").write_text(
            "book_id: demo\ntitle: Demo\nstage: writing\nlast_committed_ch: 1\nqc_profile: mystery\nblockers: []\n",
            encoding="utf-8",
        )
        self._edit("outline/units/v01-U1.yaml", "on_stage: [zhu-jue, lin-xue]\npov: zhu-jue\n", "")
        self._edit("outline/units/v01-U1.yaml", 'state_deltas: ["主角: 被辱→声望回升"]', 'state_deltas: ["zhu-jue: 被辱→声望回升"]')
        self._edit("outline/units/v01-U1.yaml", "status: accepted", "status: reviewed")
        (book / "canon/cast/zhu-jue.md").write_text(
            CAST_ZHUJUE.replace("`role`: protagonist\n", "").replace("- 本职（一句话）：落魄捕快", "- 角色：protagonist"),
            encoding="utf-8",
        )
        self._edit("canon/cast/lin-xue.md", "`role`: recurring\n", "")
        self._edit("outline/volumes/v01.md", "## 本卷人物（stem；批准即 canon）\n\n- zhu-jue\n- lin-xue\n", "")
        rep = ng.run(str(self.root), "demo", "migrate", "")
        self.assertEqual(rep.verdict, "PASS", rep.format())
        facts = (book / "continuity/facts.md").read_text(encoding="utf-8")
        self.assertNotIn("## ch001", facts)
        self.assertIn("v01 → continuity/summaries/v01.md", facts)
        self.assertIn("## ch001", (book / "continuity/summaries/v01.md").read_text(encoding="utf-8"))
        state = (book / "novel-state.yaml").read_text(encoding="utf-8")
        self.assertIn("genre: 悬疑", state)
        self.assertIn("确认 genre", state)
        unit = (book / "outline/units/v01-U1.yaml").read_text(encoding="utf-8")
        self.assertIn("on_stage: [zhu-jue]", unit)
        self.assertIn('pov: ""', unit)
        self.assertIn("`role`: protagonist", (book / "canon/cast/zhu-jue.md").read_text(encoding="utf-8"))
        self.assertIn("`role`: recurring", (book / "canon/cast/lin-xue.md").read_text(encoding="utf-8"))
        vol = (book / "outline/volumes/v01.md").read_text(encoding="utf-8")
        self.assertIn("## 本卷人物", vol)
        self.assertIn("- zhu-jue", vol)
        self.assertTrue(list((book / "continuity/commits").glob("migrate-*.md")))
        facts_new = facts.replace("| 主角 |", "| zhu-jue |")
        (book / "continuity/facts.md").write_text(facts_new, encoding="utf-8")
        rep2 = ng.run(str(self.root), "demo", "postcommit", "v01-U1")
        self.assertEqual(rep2.verdict, "PASS", rep2.format())
        rep3 = ng.run(str(self.root), "demo", "doctor", "")
        self.assertEqual(rep3.verdict, "PASS", rep3.format())
        self.assertFalse(any(f["check"] == "migrate" for f in rep3.findings), rep3.format())

    def test_doctor_advises_migrate_on_old_layout(self):
        self._edit("continuity/facts.md", "## Chapter summaries\n", "## Chapter summaries\n" + POST_SUMMARY)
        rep = ng.run(str(self.root), "demo", "doctor", "")
        self.assertEqual(rep.verdict, "PASS", rep.format())
        self.assertTrue(any(f["check"] == "migrate" and "summaries" in f["message"] for f in rep.findings), rep.format())

    def test_parse_volume_helpers(self):
        rows = ng.parse_volume_index(VOLUME, "v01")
        self.assertEqual(rows[0]["unit_id"], "v01-U1")
        self.assertEqual(rows[0]["chapter_range"], [1, 1])
        self.assertEqual(rows[0]["next_hook_type"], "未兑现承诺")
        self.assertEqual(ng.parse_volume_cast(VOLUME), ["zhu-jue", "lin-xue"])
        short = "| U3 | 7-9 | 功能 | | 倒计时 |"
        self.assertEqual(ng.parse_volume_index("## 单元索引\n" + short, "v02")[0]["unit_id"], "v02-U3")

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

    def test_yaml_fallback_parses_unit_outline(self):
        data = ng._load_yaml_map_fallback(OUTLINE)
        self.assertEqual([s["id"] for s in data["scenes"]], ["S1", "S2"])
        self.assertEqual(data["scenes"][0]["beat"], "建立期待")
        self.assertEqual(data["scenes"][1]["must_land"], ["亮出腰牌"])
        self.assertEqual(data["chapters"][0]["word_share"], 4000)
        self.assertEqual(data["next_hook"]["out"], "明日午时当众验骨")
        self.assertEqual(data["state_deltas"], ["主角: 被辱→声望回升"])
        self.assertEqual(data["info_control"]["foreshadowing"], ["FS-001: plant"])
        self.assertEqual(data["forbidden"], ["宿敌真身"])

    def test_yaml_fallback_nested_list_block_and_comment(self):
        text = """unit_id: v01-U1  # 注释里的: 冒号不能吃掉值
scenes:
  - id: S1
    must_land:
      - 发现
      - 报案
    beat: 建立期待
  - id: S2
    beat: 兑现
note: |
  第一行
  第二行: 保留冒号
"""
        data = ng._load_yaml_map_fallback(text)
        self.assertEqual(data["unit_id"], "v01-U1")
        self.assertEqual(data["scenes"][0]["must_land"], ["发现", "报案"])
        self.assertEqual(data["scenes"][0]["beat"], "建立期待")
        self.assertEqual(data["scenes"][1]["id"], "S2")
        self.assertIn("第二行: 保留冒号", data["note"])

    def test_kb_cites_resolve(self):
        errors = ng.plugin_kb_cite_errors()
        self.assertEqual(errors, [], "\n".join(errors))

    def test_kb_cite_unknown_title_and_section(self):
        bad = ng.kb_cite_errors({
            "knowledge/a.md": "# 节奏与结构\n\n见「不存在的篇」。\n见「节奏与结构 → 没有这节」。\n",
        })
        self.assertEqual(len(bad), 2, bad)
        ok = ng.kb_cite_errors({
            "knowledge/a.md": "# 节奏与结构\n\n## 矛盾链（平 → 爽）\n\n见「矛盾链」。见「节奏与结构 → 矛盾链」。\n",
            "knowledge/b.md": "# 悬疑\n\n改查「节奏与结构」。才查「节奏与结构」。\n",
        })
        self.assertEqual(ok, [], ok)

    def test_preflight_without_pyyaml(self):
        import builtins

        real_import = builtins.__import__

        def fake_import(name, *args, **kwargs):
            if name == "yaml":
                raise ImportError("no yaml")
            return real_import(name, *args, **kwargs)

        builtins.__import__ = fake_import
        sys.modules.pop("yaml", None)
        try:
            rep = ng.run(str(self.root), "demo", "preflight", "v01-U1")
        finally:
            builtins.__import__ = real_import
        self.assertEqual(rep.verdict, "PASS", rep.format())


if __name__ == "__main__":
    unittest.main()
