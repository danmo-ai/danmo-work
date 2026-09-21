import assert from 'node:assert/strict'
import {
  applyUnitOutline,
  buildConstrainedPrefill,
  buildNovelStagePrefill,
  buildUnitEntries,
  buildUnitPhases,
  canRunAction,
  computeBookPipeline,
  countPlainChars,
  formatLoadProtocol,
  inferChapterNextAction,
  inferUnitPhase,
  isNovelUnitOutlineName,
  isNovelUnitProseName,
  isVolumeOutlineName,
  mergeVolumeOutlineFiles,
  nextVolumeNumber,
  novelActionSkillId,
  novelUnitOutlinePath,
  novelUnitProsePath,
  parseBookOutlineVolumeRows,
  parseChapterRange,
  parseNovelStateExtended,
  parseNovelStateYaml,
  parseReviewVerdict,
  parseUnitOutlineYaml,
  parseVolumeUnitRows,
  setupDocLabel,
  splitUnitProseSections,
  formatChapterPlain,
  formatUnitProsePlain,
  volumeNumFromName,
} from '../src/types/novel-workbench.ts'

const yaml = `
title: "星际旅店"
stage: writing
last_committed_ch: 5
next_action: "审本单元"
active_unit: v01-U1
qc_profile: male_power
continuation_mode: false
gates:
  knowledge: pass
  asset: unknown
  qc: fail
blockers:
  - 缺反派人物卡
`
const summary = parseNovelStateYaml(yaml)
assert.equal(summary.title, '星际旅店')
assert.equal(summary.stage, 'writing')
assert.equal(summary.lastCommittedCh, 5)

const commented = `
title: "女警她脑子里有个刑侦泰斗"
stage: writing  # 迁移批注释不该进书架
last_committed_ch: 33  # 很长的定稿说明
qc_profile: mystery  # 刑侦
`
const commentedState = parseNovelStateYaml(commented)
assert.equal(commentedState.title, '女警她脑子里有个刑侦泰斗')
assert.equal(commentedState.stage, 'writing')
assert.equal(commentedState.lastCommittedCh, 33)
assert.equal(parseNovelStateExtended(commented).qcProfile, 'mystery')

const ext = parseNovelStateExtended(yaml)
assert.equal(ext.qcProfile, 'male_power')
assert.equal(ext.activeUnit, 'v01-U1')
assert.equal(ext.gates.knowledge, 'pass')
assert.equal(ext.gates.qc, 'fail')
assert.equal(ext.blockers.length, 1)
assert.equal(parseReviewVerdict('### VERDICT\nPASS'), 'PASS')
assert.equal(parseReviewVerdict('### VERDICT\nFAIL'), 'FAIL')

const outlineRaw = `unit_id: v01-U1
title_working: 夜雨
status: accepted
function: 立冲突
desire: 活下来
obstacle: 追兵
pleasure: 反杀
word_target: 8000
next_hook:
  type: 未兑现承诺
  out: 明日验骨
chapter_range: [1, 2]
scenes:
  - id: S1
    beat: 建立期待
    chapter: 1
  - id: S2
    beat: 加压
    chapter: 1
chapters:
  - chapter: 1
    title_working: 夜雨
    cut_hook: 敲门
    word_share: 4000
  - chapter: 2
    title_working: 上门
    cut_hook: 明日验骨
    word_share: 4000
`
const parsed = parseUnitOutlineYaml(outlineRaw)
assert.equal(parsed.unitId, 'v01-U1')
assert.equal(parsed.status, 'accepted')
assert.equal(parsed.functionText, '立冲突')
assert.equal(parsed.desire, '活下来')
assert.equal(parsed.pleasure, '反杀')
assert.equal(parsed.wordTarget, '8000')
assert.equal(parsed.hookType, '未兑现承诺')
assert.equal(parsed.hookOut, '明日验骨')
assert.equal(parsed.scenes.length, 2)
assert.equal(parsed.scenes[0].id, 'S1')
assert.equal(parsed.scenes[0].chapter, 1)
assert.equal(parsed.chapters.length, 2)
assert.equal(parsed.chapters[0].cutHook, '敲门')
assert.equal(parsed.chapters[1].wordShare, '4000')

const built = buildUnitEntries(
  [{ name: 'v01-U1.yaml', path: 'novel/b/outline/units/v01-U1.yaml', isDir: false }],
  [
    { name: 'v01-U1.md', path: 'novel/b/units/v01-U1.md', isDir: false },
    { name: 'v01-U2.md', path: 'novel/b/units/v01-U2.md', isDir: false },
  ],
)
assert.deepEqual(built.map((e) => e.unitId), ['v01-U1', 'v01-U2'])
assert.equal(isNovelUnitOutlineName('v01-U1.yaml'), true)
assert.equal(isNovelUnitProseName('v01-U1.md'), true)
assert.equal(isNovelUnitOutlineName('ch001-outline.yaml'), false)

const u1 = applyUnitOutline(built[0], outlineRaw)
assert.equal(u1.chapterFrom, 1)
assert.equal(u1.chapterTo, 2)
assert.equal(inferUnitPhase(u1, 5, outlineRaw, '### VERDICT\nPASS'), 'committed')
assert.equal(inferUnitPhase({ ...u1, prose: null }, 0, outlineRaw), 'contract_ready')
assert.equal(inferUnitPhase({ ...u1, prose: u1.prose }, 0, outlineRaw.replace('accepted', 'drafted')), 'drafted')
assert.equal(inferChapterNextAction('contract_ready'), 'write')
assert.equal(inferChapterNextAction('drafted'), 'review')
assert.equal(inferChapterNextAction('review_fail'), 'review')
assert.equal(inferChapterNextAction('review_pass'), 'commit')

const phases = buildUnitPhases(
  built,
  0,
  { 'v01-U1': outlineRaw },
  {},
)
assert.equal(phases['v01-U1'], 'drafted')
assert.equal(phases['v01-U2'], 'drafted')

const ctx = {
  bookId: 'star-inn',
  state: ext,
  entries: [u1, { ...built[1], chapterFrom: 3, chapterTo: 5 }],
  unitPhases: { 'v01-U1': 'committed', 'v01-U2': 'contract_ready' },
  castFileCount: 0,
  hasBookOutline: true,
  hasVolumeOutline: true,
  hasBatchFreezeFile: false,
  batchFreezeFrozen: false,
}
const pipe = computeBookPipeline(ctx)
assert.equal(pipe.progress.committed, 1)
assert.equal(pipe.progress.totalWithContract, 2)
assert.equal(pipe.primaryUnit, 'v01-U2')
assert.equal(pipe.primaryAction, 'write')

const writeBlocked = canRunAction('write', ctx, 'v01-U2')
assert.equal(writeBlocked.allowed, false)
assert.ok(writeBlocked.blockers.includes('blocker.noCast'))

const writeOk = canRunAction('write', { ...ctx, castFileCount: 2 }, 'v01-U2')
assert.equal(writeOk.allowed, true)

const reviewOk = canRunAction('review', {
  ...ctx,
  castFileCount: 2,
  unitPhases: { 'v01-U1': 'committed', 'v01-U2': 'drafted' },
}, 'v01-U2')
assert.equal(reviewOk.allowed, true)

const commitBlocked = canRunAction('commit', {
  ...ctx,
  castFileCount: 2,
  unitPhases: { 'v01-U2': 'drafted' },
}, 'v01-U2')
assert.ok(commitBlocked.blockers.includes('blocker.needReviewPass'))

const constrained = buildConstrainedPrefill('write', {
  bookId: 'star-inn',
  unitId: 'v01-U1',
  unitPath: 'novel/star-inn/units/v01-U1.md',
}, pipe, [])
assert.ok(constrained.includes('【任务】'))
assert.ok(constrained.includes('技能 novel-write · 意图 write'))
assert.ok(constrained.includes('preflight --unit'))
assert.ok(constrained.includes('Intent→Load'))
assert.ok(!constrained.includes('chapter-write.md'))
assert.ok(constrained.includes('delegate_agent.goal'))

assert.equal(novelActionSkillId('init'), 'novel-setup')
assert.equal(novelActionSkillId('write'), 'novel-write')
assert.equal(novelActionSkillId('review'), 'novel-review')
assert.equal(novelActionSkillId('commit'), 'novel-review')
assert.ok(formatLoadProtocol('write').includes('gate preflight --unit'))
assert.ok(!formatLoadProtocol('write').includes('chapter-write.md'))

const stages = [
  'init',
  'outline',
  'volume',
  'assets',
  'goldfinger',
  'contract',
  'write',
  'continue',
  'expand',
  'review',
  'polish',
  'commit',
  'review-polish-commit',
  'continuation',
  'preflight',
]
for (const action of stages) {
  const text = buildNovelStagePrefill(/** @type {any} */ (action), {
    bookId: 'star-inn',
    unitId: 'v01-U1',
    unitPath: 'novel/star-inn/units/v01-U1.md',
  })
  assert.ok(text.trim().length > 0, action)
  assert.ok(!text.includes('章纲'), `${action} must not say 章纲`)
  assert.ok(!text.includes('章合同'), `${action} must not say 章合同`)
  assert.ok(!text.includes('chapters/ch'), `${action} must not point at chapter files`)
  assert.ok(!text.includes('按 read_skill'), action)
  const constrainedText = buildConstrainedPrefill(/** @type {any} */ (action), {
    bookId: 'star-inn',
    unitId: 'v01-U1',
  })
  assert.ok(constrainedText.includes(`技能 ${novelActionSkillId(/** @type {any} */ (action))} · 意图 ${action}`), action)
  assert.ok(!constrainedText.includes('/references/'), action)
  assert.ok(!constrainedText.includes('/assets/templates/'), action)
}

const contractPrefill = buildNovelStagePrefill('contract', { bookId: 'star-inn', unitId: 'v01-U1' })
assert.ok(contractPrefill.includes('单元细纲'))
assert.ok(contractPrefill.includes('outline/units/v01-U1.yaml'))
assert.ok(contractPrefill.includes('active_unit'))
assert.equal(novelUnitOutlinePath('star-inn', 'v01-U1'), 'novel/star-inn/outline/units/v01-U1.yaml')
assert.equal(novelUnitProsePath('star-inn', 'v01-U1'), 'novel/star-inn/units/v01-U1.md')

const writePrefill = buildNovelStagePrefill('write', { bookId: 'star-inn', unitId: 'v01-U1' })
assert.ok(writePrefill.includes('units/v01-U1.md'))
assert.ok(writePrefill.includes('preflight --unit'))
assert.ok(writePrefill.includes('---'))
assert.ok(writePrefill.includes('停下'))

const preflightPrefill = buildNovelStagePrefill('preflight', { bookId: 'star-inn', unitId: 'v01-U1' })
assert.ok(preflightPrefill.includes('preflight --unit'))

const md = `## 第1章 夜雨

甲

乙

---

## 第2章 上门

丙
`
const sections = splitUnitProseSections(md)
assert.equal(sections.length, 2)
assert.equal(sections[0].chapter, 1)
assert.equal(sections[0].title, '夜雨')
assert.ok(sections[0].body.includes('甲'))
assert.ok(sections[0].body.includes('乙'))
assert.ok(!sections[0].body.includes('---'))
assert.equal(sections[1].title, '上门')
assert.ok(countPlainChars(sections[1].body) > 0)
assert.equal(formatChapterPlain(sections[0]), '第1章 夜雨\n\n甲\n\n乙\n')
assert.ok(formatUnitProsePlain(sections).includes('\n---\n\n第2章 上门\n'))

assert.equal(nextVolumeNumber([]), 1)
assert.equal(volumeNumFromName('v12-补.md'), 12)
assert.equal(volumeNumFromName('volume01-chapter-index.md'), 1)
assert.equal(isVolumeOutlineName('book_outline.md'), false)
assert.equal(isVolumeOutlineName('volume01-chapter-index.md'), true)
assert.deepEqual(
  mergeVolumeOutlineFiles(
    [
      { name: 'book_outline.md', path: 'o/book_outline.md', isDir: false },
      { name: 'volume01-chapter-index.md', path: 'o/volume01-chapter-index.md', isDir: false },
    ],
    [{ name: 'v02.md', path: 'o/volumes/v02.md', isDir: false }],
  ).map((n) => n.name),
  ['v02.md', 'volume01-chapter-index.md'],
)
assert.deepEqual(
  parseBookOutlineVolumeRows(`## 分卷结构\n\n| 卷 | 卷目标 | 卷高潮 | 主反转 |\n|----|--------|--------|--------|\n| v01 | 拿到碎片 | 宗门大比 | 师尊是敌 |\n`),
  [{ vol: 'v01', goal: '拿到碎片', climax: '宗门大比', twist: '师尊是敌' }],
)
assert.equal(
  parseVolumeUnitRows(`| 单元 | 章范围 | 功能（本段必须完成） | 由上一单元如何导致 | 主爽点形态 |\n| U1 | ch001-ch008 | 开局夺权 | | 智斗 |\n`)[0].purpose,
  '开局夺权',
)
const unitCards = parseVolumeUnitRows(`## 剧情单元

### 剧情单元 U1

- 单元ID：\`v01-U1\`
- 章范围：ch1-ch5
- 单元功能（本段必须完成）：开局立冲突

### 剧情单元 U2

- 单元ID：\`v01-U2\`
- 章范围：ch6-ch10
- 单元功能（本段必须完成）：宗门大比

## 情绪与人物弧
`)
assert.equal(unitCards.length, 2)
assert.equal(unitCards[0].id, 'v01-U1')
assert.equal(unitCards[0].purpose, '开局立冲突')
assert.deepEqual(parseChapterRange('ch001-ch008'), { from: 1, to: 8 })
assert.equal(setupDocLabel('book-bible.md'), 'bible')

console.log('novel-workbench helpers ok')
