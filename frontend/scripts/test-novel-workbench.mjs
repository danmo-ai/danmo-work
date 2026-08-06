import assert from 'node:assert/strict'
import {
  buildNovelStagePrefill,
  chapterNumFromName,
  nextChapterNumber,
  parseNovelStateYaml,
  sortChapterNodes,
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
assert.equal(chapterNumFromName('notes.md'), null)

const sorted = sortChapterNodes([
  { name: 'ch10.md', path: 'novel/b/chapters/ch10.md', isDir: false },
  { name: 'ch2.md', path: 'novel/b/chapters/ch2.md', isDir: false },
  { name: 'drafts', path: 'novel/b/chapters/drafts', isDir: true },
  { name: 'readme.txt', path: 'novel/b/chapters/readme.txt', isDir: false },
])
assert.deepEqual(sorted.map((n) => n.name), ['ch2.md', 'ch10.md'])

assert.equal(
  nextChapterNumber(3, [
    { name: 'ch001.md', path: 'a', isDir: false },
    { name: 'ch005.md', path: 'b', isDir: false },
  ]),
  6,
)

const init = buildNovelStagePrefill('init', {})
assert.ok(init.includes('开一本新书'))
assert.ok(!init.includes('delegate_agent'))
assert.ok(!init.includes('请委派'))

const cont = buildNovelStagePrefill('continue', { bookId: 'star-inn' })
assert.ok(cont.includes('novel/star-inn/novel-state.yaml'))
assert.ok(!cont.includes('delegate_agent'))

const write = buildNovelStagePrefill('write', { bookId: 'star-inn', chapter: 4 })
assert.ok(write.includes('ch004.md'))

const review = buildNovelStagePrefill('review', {
  bookId: 'star-inn',
  chapterPath: 'novel/star-inn/chapters/ch003.md',
})
assert.ok(review.includes('novel/star-inn/chapters/ch003.md'))
assert.ok(review.includes('reviews/'))
assert.ok(!review.includes('请委派'))

console.log('novel-workbench helpers ok')
