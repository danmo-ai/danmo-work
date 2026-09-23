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
  castLintIssues,
  inferBookPipelinePhase,
  inferUnitPhase,
  parseCastCard,
  parseVolumeCast,
  selectPrimaryAction,
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
assert.deepEqual(parsed.onStage, [])
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
const reviewed = outlineRaw.replace('status: accepted', 'status: reviewed')
assert.equal(inferUnitPhase(u1, 5, reviewed, '### VERDICT\nPASS'), 'finalized')
assert.equal(inferUnitPhase(u1, 5, outlineRaw, '### VERDICT\nPASS'), 'drafted')
assert.equal(inferUnitPhase({ ...u1, prose: null }, 0, outlineRaw), 'ready')
assert.equal(inferUnitPhase({ ...u1, prose: null }, 0, 'unit_id: v01-U1\nstatus: proposed\nchapter_range: [1, 2]\n'), 'pending_outline')
assert.equal(inferUnitPhase({ ...u1, prose: u1.prose }, 0, outlineRaw.replace('accepted', 'drafted')), 'drafted')
assert.equal(inferUnitPhase(u1, 0, outlineRaw, '### VERDICT\nFAIL'), 'review_fail')

const phases = buildUnitPhases(
  built,
  0,
  { 'v01-U1': outlineRaw },
  {},
)
assert.equal(phases['v01-U1'], 'drafted')
assert.equal(phases['v01-U2'], 'drafted')

function unitEntry(id, phase) {
  const meta = id.match(/v(\d+)-U(\d+)/)
  const hasProse = phase === 'drafted' || phase === 'review_fail' || phase === 'finalized'
  return {
    unitId: id,
    volume: Number(meta[1]),
    index: Number(meta[2]),
    label: id,
    chapterFrom: 1,
    chapterTo: 3,
    outline: { name: `${id}.yaml`, path: '', isDir: false },
    prose: hasProse ? { name: `${id}.md`, path: '', isDir: false } : null,
  }
}

const clean = { ...ext, blockers: [] }
function book(partial) {
  return {
    bookId: 'star-inn',
    state: clean,
    entries: [],
    unitPhases: {},
    unitOutlines: {},
    volumes: [{ volume: 1, fileName: 'v01.md', cast: ['lin'], rows: [] }],
    cast: [],
    hasBookOutline: true,
    legacy: false,
    ...partial,
  }
}

const linCard = `# 林雪
\`status\`: canon
\`role\`: protagonist

## 功能
- 退场：仍在场

## 四件套（protagonist / volume_antagonist 必填）
- 欲望（此刻要什么，可观察）：活下去

## 知识边界
- 不知：凶手是谁

## 三锚点（气质锁定）
- **视觉**：左耳银钉
- **语言**：短句
- **行为**：摸戒指

## 语言习惯
- 口头禅（≤2）：行吧

## 台词样例
- 压力下：你先说。
- 日常：喝茶。
- 掩饰 / 说谎：没什么。

## 关系
| 对方 | 类型 | 当前质态 | 最近变化点 | 下一预期节点 |
|------|------|----------|------------|--------------|
| zhou | 对手 | 猜疑 | v01-U1 | 摊牌 |
`
const lin = parseCastCard(linCard, 'lin')
assert.equal(lin.status, 'canon')
assert.equal(lin.role, 'protagonist')
assert.equal(lin.visualAnchor, '左耳银钉')
assert.equal(lin.catchphrase, '行吧')
assert.equal(lin.dialogue.pressure, '你先说。')
assert.equal(lin.unknown, '凶手是谁')
assert.deepEqual(lin.missing, [])
assert.equal(lin.relations[0].target, 'zhou')
assert.equal(lin.relations[0].current, '猜疑')

const recurring = parseCastCard(`# 周
\`status\`: candidate
\`role\`: recurring
## 功能
- 与主角相交点（recurring 必填）：
- 退场：
## 三锚点
- **视觉**：
`, 'zhou')
assert.ok(recurring.missing.includes('欲望'))
assert.ok(recurring.missing.includes('口头禅'))
assert.ok(recurring.missing.includes('退场'))

const zhou = parseCastCard(`# 周
\`status\`: canon
\`role\`: recurring
## 功能
- 欲望（此刻要什么）：自保
- 与主角相交点（recurring 必填）：搭档
- 退场：仍在场
## 三锚点
- **视觉**：旧风衣
## 语言习惯
- 口头禅（≤2）：得了
## 台词样例
- 压力下：别问。
## 关系
| 对方 | 类型 | 当前质态 | 最近变化点 | 下一预期节点 |
| lin | 搭档 | 信任 | v01-U1 | |
`, 'zhou')
assert.deepEqual(castLintIssues([lin, zhou]), [])
assert.ok(castLintIssues([lin]).some((x) => x.startsWith('cast.unknownStem:')))
assert.ok(castLintIssues([lin, { ...zhou, relations: [] }]).some((x) => x.startsWith('cast.missingBackEdge:')))

assert.deepEqual(
  parseVolumeCast(`## 本卷人物（stem；批准即 canon）\n- lin\n- \`zhou\`\n\n## 情绪与人物弧\n- 别人`),
  ['lin', 'zhou'],
)
const indexRows = parseVolumeUnitRows(`## 单元索引（只索引，不展开）\n\n| unit_id | 章范围 | 一句话功能 | 本单元终局边界（禁碰） | 下一单元钩子类型 |\n|---------|--------|------------|------------------------|------------------|\n| U1 | ch1-ch3 | 立冲突 | 禁终局 | 信息缺口 |\n`, 1)
assert.equal(indexRows[0].id, 'v01-U1')
assert.equal(indexRows[0].purpose, '立冲突')
assert.equal(indexRows[0].endgameBoundary, '禁终局')
assert.equal(indexRows[0].hookType, '信息缺口')

const planning = selectPrimaryAction(book({ volumes: [] }))
assert.equal(inferBookPipelinePhase(book({ volumes: [] })), 'planning')
assert.equal(planning.action, 'plan')
assert.equal(planning.volume, 1)

const approve = selectPrimaryAction(book({ entries: [] }))
assert.equal(approve.action, 'plan')
assert.equal(approve.volume, 1)

const proposedIds = ['v01-U1', 'v01-U2', 'v01-U3', 'v01-U4', 'v01-U5']
const outlining = book({
  entries: proposedIds.map((id) => unitEntry(id, 'pending_outline')),
  unitPhases: Object.fromEntries(proposedIds.map((id) => [id, 'pending_outline'])),
})
assert.equal(inferBookPipelinePhase(outlining), 'outlining')
const batch = selectPrimaryAction(outlining, 'v01-U5')
assert.equal(batch.action, 'outline-batch')
assert.deepEqual(batch.batchUnits, proposedIds.slice(0, 4))

const readyBook = book({
  entries: [unitEntry('v01-U1', 'ready')],
  unitPhases: { 'v01-U1': 'ready' },
  unitOutlines: { 'v01-U1': { ...parsed, onStage: ['lin'], pov: 'lin' } },
  cast: [lin],
})
assert.equal(inferBookPipelinePhase(readyBook), 'units')
const writeDecision = selectPrimaryAction(readyBook, 'v01-U1')
assert.equal(writeDecision.action, 'write')
assert.equal(writeDecision.unitId, 'v01-U1')
assert.equal(writeDecision.allowed, true)

const noCast = selectPrimaryAction({ ...readyBook, cast: [] }, 'v01-U1')
assert.equal(noCast.allowed, false)
assert.ok(noCast.blockers.includes('blocker.noCast'))

const candidate = selectPrimaryAction({
  ...readyBook,
  cast: [{ ...lin, status: 'candidate' }],
}, 'v01-U1')
assert.ok(candidate.blockers.includes('blocker.noProtagonist'))
assert.ok(candidate.blockers.includes('blocker.candidateOnStage:lin'))

const outside = selectPrimaryAction({
  ...readyBook,
  volumes: [{ volume: 1, fileName: 'v01.md', cast: ['other'], rows: [] }],
}, 'v01-U1')
assert.ok(outside.blockers.includes('blocker.castOutsideVolume:lin'))

const draftedBook = book({
  entries: [unitEntry('v01-U1', 'drafted'), unitEntry('v01-U2', 'ready')],
  unitPhases: { 'v01-U1': 'drafted', 'v01-U2': 'ready' },
  cast: [lin],
})
assert.equal(selectPrimaryAction(draftedBook, 'v01-U1').action, 'finalize')
assert.equal(selectPrimaryAction(draftedBook, 'v01-U1').unitId, 'v01-U1')

const jumped = book({
  entries: [unitEntry('v01-U1', 'finalized'), unitEntry('v01-U2', 'ready')],
  unitPhases: { 'v01-U1': 'finalized', 'v01-U2': 'ready' },
  unitOutlines: { 'v01-U2': { ...parsed, unitId: 'v01-U2', onStage: ['lin'] } },
  cast: [lin],
})
assert.equal(selectPrimaryAction(jumped, 'v01-U1').action, 'write')
assert.equal(selectPrimaryAction(jumped, 'v01-U1').unitId, 'v01-U2')

const done = book({
  entries: [unitEntry('v01-U1', 'finalized')],
  unitPhases: { 'v01-U1': 'finalized' },
})
assert.equal(inferBookPipelinePhase(done), 'planning')
assert.equal(selectPrimaryAction(done).action, 'next-volume')
assert.equal(selectPrimaryAction(done).volume, 2)
assert.equal(selectPrimaryAction({ ...done, legacy: true }).action, 'migrate')

const pipe = computeBookPipeline(jumped, 'v01-U1')
assert.equal(pipe.phase, 'units')
assert.equal(pipe.progress.finalized, 1)
assert.equal(pipe.progress.outlined, 2)
assert.equal(pipe.progress.total, 2)
assert.equal(pipe.primary.action, 'write')
assert.equal(pipe.gates.asset, 'pass')
assert.equal(computeBookPipeline(book({ volumes: [], cast: [{ ...lin, status: 'candidate' }] })).gates.asset, 'unknown')

const writeBlocked = canRunAction('write', { ...readyBook, cast: [] }, 'v01-U1')
assert.equal(writeBlocked.allowed, false)
assert.ok(writeBlocked.blockers.includes('blocker.noCast'))
assert.equal(canRunAction('write', readyBook, 'v01-U1').allowed, true)
assert.equal(canRunAction('review', draftedBook, 'v01-U1').allowed, true)
assert.ok(canRunAction('expand', readyBook, 'v01-U1').blockers.includes('blocker.needDraft'))
assert.ok(canRunAction('finalize', { ...draftedBook, unitPhases: { 'v01-U1': 'finalized' } }, 'v01-U1').blockers.includes('blocker.alreadyFinalized'))

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
assert.equal(novelActionSkillId('migrate'), 'novel-setup')
assert.equal(novelActionSkillId('plan'), 'novel-plan')
assert.equal(novelActionSkillId('write'), 'novel-write')
assert.equal(novelActionSkillId('outline-batch'), 'novel-write')
assert.equal(novelActionSkillId('finalize'), 'novel-review')
assert.equal(novelActionSkillId('review'), 'novel-review')
assert.ok(formatLoadProtocol('write').includes('gate preflight --unit'))
assert.ok(formatLoadProtocol('outline-batch').includes('lint-units'))
assert.ok(formatLoadProtocol('finalize').includes('qc-pack'))
assert.ok(!formatLoadProtocol('write').includes('chapter-write.md'))

const stages = [
  'init',
  'migrate',
  'plan',
  'next-volume',
  'outline-batch',
  'contract-one',
  'write',
  'finalize',
  'expand',
  'review',
  'polish',
  'cast-fix',
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

const contractPrefill = buildNovelStagePrefill('contract-one', { bookId: 'star-inn', unitId: 'v01-U1' })
assert.ok(contractPrefill.includes('细纲'))
assert.ok(contractPrefill.includes('outline/units/v01-U1.yaml'))
assert.ok(contractPrefill.includes('lint-units'))
const planPrefill = buildNovelStagePrefill('plan', { bookId: 'star-inn', volume: 1, volumeOutlineExists: true })
assert.ok(planPrefill.includes('accept-volume'))
assert.ok(!planPrefill.includes('kb-novel'))
assert.equal(novelUnitOutlinePath('star-inn', 'v01-U1'), 'novel/star-inn/outline/units/v01-U1.yaml')
assert.equal(novelUnitProsePath('star-inn', 'v01-U1'), 'novel/star-inn/units/v01-U1.md')

const writePrefill = buildNovelStagePrefill('write', { bookId: 'star-inn', unitId: 'v01-U1' })
assert.ok(writePrefill.includes('units/v01-U1.md'))
assert.ok(writePrefill.includes('preflight --unit'))
assert.ok(writePrefill.includes('---'))
assert.ok(writePrefill.includes('停下'))
assert.ok(writePrefill.includes('### CONTEXT'))

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
