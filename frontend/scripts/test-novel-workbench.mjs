import assert from 'node:assert/strict'
import {
  buildChapterEntries,
  buildChapterPhases,
  buildConstrainedPrefill,
  buildNovelStagePrefill,
  canRunAction,
  chapterNumFromName,
  computeBookPipeline,
  inferChapterNextAction,
  inferChapterPhase,
  inferNovelBookNextStep,
  isNovelChapterPath,
  isNovelContractName,
  isNovelContractPath,
  nextChapterNumber,
  nextVolumeNumber,
  parseBatchFreezeYaml,
  parseContractYaml,
  parseNovelStateExtended,
  parseBookOutlineVolumeRows,
  parseNovelStateYaml,
  parseReviewVerdict,
  parseChapterRange,
  parseVolumeUnitRows,
  mergeVolumeOutlineFiles,
  isVolumeOutlineName,
  setupDocLabel,
  sortChapterNodes,
  volumeNumFromName,
} from '../src/types/novel-workbench.ts'

const yaml = `
title: "星际旅店"
stage: writing
last_committed_ch: 3
next_action: "写第4章钩子"
qc_profile: male_power
continuation_mode: false
frozen_batch:
  from: 1
  to: 8
  frozen_at: "2026-01-01"
artifacts:
  batch_freeze: frozen
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
assert.equal(summary.lastCommittedCh, 3)

const ext = parseNovelStateExtended(yaml)
assert.equal(ext.qcProfile, 'male_power')
assert.equal(ext.frozenBatch?.from, 1)
assert.equal(ext.frozenBatch?.to, 8)
assert.equal(ext.batchFreezeArtifact, 'frozen')
assert.equal(ext.gates.knowledge, 'pass')
assert.equal(ext.gates.qc, 'fail')
assert.equal(ext.blockers.length, 1)

assert.equal(parseContractYaml('status: accepted').status, 'accepted')
assert.equal(parseContractYaml('title_working: 孙馆长\nstatus: proposed').title, '孙馆长')
assert.equal(parseContractYaml('unit_id: v04-U2\nstatus: proposed').unitId, 'v04-U2')
assert.equal(parseReviewVerdict('### VERDICT\nPASS'), 'PASS')
assert.equal(parseReviewVerdict('### VERDICT\nFAIL'), 'FAIL')
assert.equal(parseBatchFreezeYaml('status: frozen').status, 'frozen')

const entryCommitted = {
  chapter: 1,
  label: 'ch001',
  prose: { name: 'ch001.md', path: 'a', isDir: false },
  contract: { name: 'ch001-contract.yaml', path: 'b', isDir: false },
}
assert.equal(
  inferChapterPhase(entryCommitted, 3, 'status: reviewed', '### VERDICT\nPASS'),
  'committed',
)
const entryReady = {
  chapter: 2,
  label: 'ch002',
  prose: null,
  contract: { name: 'ch002-contract.yaml', path: 'b', isDir: false },
}
assert.equal(inferChapterPhase(entryReady, 0, 'status: accepted', null), 'contract_ready')
assert.equal(inferChapterNextAction('contract_ready'), 'write')
assert.equal(inferChapterNextAction('review_pass'), 'commit')

const entries = buildChapterEntries(
  [
    { name: 'ch001-contract.yaml', path: 'novel/b/chapters/ch001-contract.yaml', isDir: false },
    { name: 'ch002-contract.yaml', path: 'novel/b/chapters/ch002-contract.yaml', isDir: false },
    { name: 'ch002.md', path: 'novel/b/chapters/ch002.md', isDir: false },
  ],
  [],
)
const phases = buildChapterPhases(entries, 0, { 1: 'status: proposed' }, {})
assert.equal(phases[1], 'contract_draft')
assert.equal(phases[2], 'drafted')

const ctx = {
  bookId: 'star-inn',
  state: ext,
  entries,
  chapterPhases: phases,
  castFileCount: 0,
  hasBookOutline: true,
  hasVolumeOutline: true,
  hasBatchFreezeFile: true,
  batchFreezeFrozen: true,
}
const pipe = computeBookPipeline(ctx)
assert.ok(pipe.primaryAction)

const writeBlocked = canRunAction('write', ctx, 2)
assert.equal(writeBlocked.allowed, false)
assert.ok(writeBlocked.blockers.includes('blocker.noCast'))

const reviewOk = canRunAction('review', { ...ctx, castFileCount: 2 }, 2)
assert.equal(reviewOk.allowed, true)

const constrained = buildConstrainedPrefill('write', { bookId: 'star-inn', chapter: 2 }, pipe, [])
assert.ok(constrained.includes('工作台约束'))
assert.ok(constrained.includes('preflight.md'))
assert.ok(constrained.includes('novel-write/'))
assert.equal(pipe.phase, 'review')

const stages = [
  'init',
  'outline',
  'volume',
  'assets',
  'goldfinger',
  'contract',
  'write',
  'continue',
  'review',
  'polish',
  'commit',
  'batch-freeze',
  'continuation',
  'batch-review',
  'preflight',
]
for (const action of stages) {
  const text = buildNovelStagePrefill(/** @type {any} */ (action), {
    bookId: 'star-inn',
    chapter: 4,
    chapterPath: 'novel/star-inn/chapters/ch004.md',
  })
  assert.ok(text.trim().length > 0, action)
  assert.ok(!text.includes('章纲'), `${action} must not say 章纲`)
  assert.ok(!text.includes('细纲'), `${action} must not say 细纲`)
}

const freezePrefill = buildNovelStagePrefill('batch-freeze', {
  bookId: 'star-inn',
  batchFrom: 1,
  batchTo: 8,
})
assert.ok(freezePrefill.includes('novel-write/references/batch-freeze.md'))
assert.ok(freezePrefill.includes('章合同'))
assert.ok(freezePrefill.includes('unit_id'))

const contractPrefill = buildNovelStagePrefill('contract', { bookId: 'star-inn', chapter: 4 })
assert.ok(contractPrefill.includes('unit_id'))
assert.ok(contractPrefill.includes('vNN-U#'))

const goldPrefill = buildNovelStagePrefill('goldfinger', { bookId: 'star-inn' })
assert.ok(goldPrefill.includes('novel-setup/assets/templates/goldfinger-card.md'))

const assetsPrefill = buildNovelStagePrefill('assets', { bookId: 'star-inn' })
assert.ok(assetsPrefill.includes('world.md'))
assert.ok(assetsPrefill.includes('cast-card.md'))

const outlinePrefill = buildNovelStagePrefill('outline', { bookId: 'star-inn' })
assert.ok(outlinePrefill.includes('终局储备'))

assert.equal(chapterNumFromName('ch001.md'), 1)
assert.equal(isNovelChapterPath('novel/b/chapters/ch003.md'), true)
assert.equal(isNovelContractPath('novel/b/chapters/ch001-contract.yaml'), true)

const sorted = sortChapterNodes([
  { name: 'ch10.md', path: 'novel/b/chapters/ch10.md', isDir: false },
  { name: 'ch2.md', path: 'novel/b/chapters/ch2.md', isDir: false },
])
assert.deepEqual(sorted.map((n) => n.name), ['ch2.md', 'ch10.md'])

assert.deepEqual(inferNovelBookNextStep(0, entries), { action: 'write', chapter: 1 })
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
assert.equal(
  parseVolumeUnitRows(`| 单元 | 章范围 | 功能（本段必须完成） | 本段主角目标 | 由上一单元如何导致 | 主爽点形态 | 禁止提前释放 | 下一单元钩子 |\n| U2 | ch009-ch016 | 宗门大比 | 保住名额 | 夺权余波 | 战力 | 金手指上限 | 师尊现身 |\n`)[0].purpose,
  '宗门大比',
)
assert.deepEqual(parseChapterRange('ch001-ch008'), { from: 1, to: 8 })
assert.equal(setupDocLabel('book-bible.md'), 'bible')
assert.equal(setupDocLabel('reveal-schedule.md'), 'reveal')
assert.equal(setupDocLabel('00-孙悟空.md'), '孙悟空')

console.log('novel-workbench helpers ok')
