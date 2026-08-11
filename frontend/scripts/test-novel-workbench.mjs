import assert from 'node:assert/strict'
import {
  buildChapterEntries,
  buildNovelStagePrefill,
  chapterNumFromName,
  inferNovelBookNextStep,
  isNovelChapterPath,
  isNovelContractName,
  isNovelContractPath,
  nextChapterNumber,
  nextVolumeNumber,
  parseNovelStateYaml,
  sortChapterNodes,
  volumeNumFromName,
} from '../src/types/novel-workbench.ts'

const yaml = `
title: "星际旅店"
stage: draft
last_committed_ch: 3
next_action: "写第4章钩子"
extra: ignore
`
const summary = parseNovelStateYaml(yaml)
assert.equal(summary.title, '星际旅店')
assert.equal(summary.stage, 'draft')
assert.equal(summary.lastCommittedCh, 3)
assert.equal(summary.nextAction, '写第4章钩子')

assert.equal(parseNovelStateYaml('title: \'\'').title, '')
assert.equal(parseNovelStateYaml('last_committed_ch: abc').lastCommittedCh, 0)

assert.equal(chapterNumFromName('ch001.md'), 1)
assert.equal(chapterNumFromName('ch12.md'), 12)
assert.equal(chapterNumFromName('ch013-contract.yaml'), 13)
assert.equal(chapterNumFromName('notes.md'), null)

assert.equal(isNovelChapterPath('novel/b/chapters/ch003.md'), true)
assert.equal(isNovelChapterPath('novel/b/book-bible.md'), false)
assert.equal(isNovelContractName('ch001-contract.yaml'), true)
assert.equal(isNovelContractName('ch001.yml'), false)
assert.equal(isNovelContractPath('novel/juanzong/chapters/ch001-contract.yaml'), true)
assert.equal(isNovelContractPath('novel/juanzong/chapters/ch001.md'), false)

const sorted = sortChapterNodes([
  { name: 'ch10.md', path: 'novel/b/chapters/ch10.md', isDir: false },
  { name: 'ch2.md', path: 'novel/b/chapters/ch2.md', isDir: false },
  { name: 'drafts', path: 'novel/b/chapters/drafts', isDir: true },
  { name: 'readme.txt', path: 'novel/b/chapters/readme.txt', isDir: false },
  { name: 'ch001-contract.yaml', path: 'novel/b/chapters/ch001-contract.yaml', isDir: false },
])
assert.deepEqual(sorted.map((n) => n.name), ['ch2.md', 'ch10.md'])

const entries = buildChapterEntries(
  [
    { name: 'ch001-contract.yaml', path: 'novel/b/chapters/ch001-contract.yaml', isDir: false },
    { name: 'ch002-contract.yaml', path: 'novel/b/chapters/ch002-contract.yaml', isDir: false },
    { name: 'ch002.md', path: 'novel/b/chapters/ch002.md', isDir: false },
  ],
  [
    { name: 'book_outline.md', path: 'novel/b/outline/book_outline.md', isDir: false },
    { name: 'ch003-contract.yaml', path: 'novel/b/outline/ch003-contract.yaml', isDir: false },
  ],
)
assert.deepEqual(
  entries.map((e) => ({
    ch: e.chapter,
    prose: e.prose?.name ?? null,
    contract: e.contract?.name ?? null,
  })),
  [
    { ch: 1, prose: null, contract: 'ch001-contract.yaml' },
    { ch: 2, prose: 'ch002.md', contract: 'ch002-contract.yaml' },
    { ch: 3, prose: null, contract: 'ch003-contract.yaml' },
  ],
)

assert.equal(
  nextChapterNumber(3, [
    { name: 'ch001.md', path: 'a', isDir: false },
    { name: 'ch005.md', path: 'b', isDir: false },
  ]),
  6,
)
assert.equal(nextChapterNumber(0, entries), 4)

const juanzongLike = buildChapterEntries([
  { name: 'ch001-contract.yaml', path: 'a', isDir: false },
  { name: 'ch002-contract.yaml', path: 'b', isDir: false },
  { name: 'ch013-contract.yaml', path: 'c', isDir: false },
])
assert.deepEqual(inferNovelBookNextStep(0, juanzongLike), { action: 'write', chapter: 1 })
assert.deepEqual(
  inferNovelBookNextStep(0, [
    {
      chapter: 1,
      label: 'ch001',
      contract: { name: 'ch001-contract.yaml', path: 'a', isDir: false },
      prose: { name: 'ch001.md', path: 'b', isDir: false },
    },
  ]),
  { action: 'continue', chapter: 2 },
)
assert.deepEqual(inferNovelBookNextStep(0, []), { action: 'contract', chapter: 1 })

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
]
for (const action of stages) {
  const text = buildNovelStagePrefill(/** @type {any} */ (action), {
    bookId: 'star-inn',
    chapter: 4,
    chapterPath: 'novel/star-inn/chapters/ch004.md',
  })
  assert.ok(text.trim().length > 0, action)
  assert.ok(!text.includes('请委派'), action)
  assert.ok(!text.startsWith('请用 delegate_agent'), action)
}

const contract = buildNovelStagePrefill('contract', { bookId: 'star-inn', chapter: 4 })
assert.ok(contract.includes('章合同'))
assert.ok(contract.includes('chapter-contract.md'))
assert.ok(contract.includes('ch004-contract.yaml'))
assert.ok(contract.includes('唯一落盘'))
assert.ok(!contract.includes('也可 outline'))

const polish = buildNovelStagePrefill('polish', {
  bookId: 'star-inn',
  chapter: 4,
  chapterPath: 'novel/star-inn/chapters/ch004.md',
})
assert.ok(polish.includes('polish-deslop.md'))

const commit = buildNovelStagePrefill('commit', { bookId: 'star-inn', chapter: 4 })
assert.ok(commit.includes('continuity-commit.md'))
assert.ok(commit.includes('phase-NN'))

const write = buildNovelStagePrefill('write', { bookId: 'star-inn', chapter: 4 })
assert.ok(write.includes('ch004.md'))
assert.ok(write.includes('accepted'))

const outline = buildNovelStagePrefill('outline', { bookId: 'star-inn' })
assert.ok(outline.includes('book-outline.md'))
assert.ok(outline.includes('volume-outline.md'))
assert.ok(outline.includes('爽点'))

const volume = buildNovelStagePrefill('volume', { bookId: 'star-inn', volume: 3 })
assert.ok(volume.includes('v03.md'))
assert.ok(volume.includes('volume-outline.md'))
assert.ok(volume.includes('章纲表'))
assert.ok(volume.includes('outline.md'))

assert.equal(volumeNumFromName('v01.md'), 1)
assert.equal(volumeNumFromName('v12-补天天道重启.md'), 12)
assert.equal(volumeNumFromName('book_outline.md'), null)
assert.equal(
  nextVolumeNumber([
    { name: 'v01.md', path: 'a', isDir: false },
    { name: 'v03-莲花生命编码.md', path: 'b', isDir: false },
    { name: 'drafts', path: 'c', isDir: true },
  ]),
  4,
)
assert.equal(nextVolumeNumber([]), 1)

console.log('novel-workbench helpers ok')
