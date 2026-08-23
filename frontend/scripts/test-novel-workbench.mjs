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
  parseNovelStateYaml,
  parseReviewVerdict,
  sortChapterNodes,
  volumeNumFromName,
} from '../src/types/novel-workbench.ts'

const yaml = `
title: "星际旅店"
stage: chapter_loop
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
assert.equal(summary.stage, 'chapter_loop')
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
}

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

console.log('novel-workbench helpers ok')
