/**
 * Unit checks for Marp-compatible slides → playable HTML.
 * Run: npm run test:slides
 */
import { webcrypto } from 'node:crypto'

if (!globalThis.crypto) {
  globalThis.crypto = webcrypto
}

const {
  parseSlidesMarkdown,
  siblingPlayableHtmlPath,
  siblingSlidesMarkdownPath,
  hashSlidesSource,
  extractSrcHashFromHtml,
  isPlayableHtmlStale,
  renderPlayableSlidesHtml,
  splitSlidesForEditor,
  joinSlidePages,
} = await import('../src/utils/slides-render.ts')

function assert(cond, msg) {
  if (!cond) throw new Error(msg)
}

const sample = `---
type: slides
title: Demo Deck
theme: default
---

# Hello

World

<!-- notes: say hi -->

---

## Bullets

- One
- Two
`

const parsed = parseSlidesMarkdown(sample)
assert(parsed.pages.length === 2, `expected 2 pages, got ${parsed.pages.length}`)
assert(parsed.title === 'Demo Deck', `title=${parsed.title}`)
assert(parsed.theme === 'default', `theme=${parsed.theme}`)
assert(parsed.pages[0].notes.includes('say hi'), 'notes missing')
assert(!parsed.pages[0].body.includes('notes:'), 'notes should be stripped from body')
assert(parsed.pages[1].body.includes('Bullets'), 'page 2 body')

assert(siblingPlayableHtmlPath('slides/foo-slides.md') === 'slides/foo-slides.html', 'sibling html')
assert(siblingSlidesMarkdownPath('slides/foo-slides.html') === 'slides/foo-slides.md', 'sibling md')

const editor = splitSlidesForEditor(sample)
assert(editor.frontmatterRaw.includes('type: slides'), 'editor frontmatter')
assert(editor.pages.length === 2, 'editor pages')
const joined = joinSlidePages(editor.pages, editor.frontmatterRaw)
assert(joined.startsWith('---\n'), 'joined keeps frontmatter')
assert(joined.includes('---\n\n## Bullets') || joined.includes('\n---\n\n## Bullets'), 'joined separators')

const hash = await hashSlidesSource(sample)
assert(/^[a-f0-9]{64}$/.test(hash), `bad hash ${hash}`)
const hash2 = await hashSlidesSource(sample)
assert(hash === hash2, 'hash unstable')

const html = renderPlayableSlidesHtml(sample, hash)
assert(html.includes('<!DOCTYPE html>'), 'doctype')
assert(html.includes(`danmo-slides-src-sha256:${hash}`), 'hash comment')
assert(html.includes('ArrowRight'), 'keyboard')
assert(html.includes('Hello'), 'slide content')
assert(extractSrcHashFromHtml(html) === hash, 'extract hash')
assert(!isPlayableHtmlStale(html, hash), 'should not be stale')
assert(isPlayableHtmlStale(null, hash), 'null is stale')
assert(isPlayableHtmlStale(html, '0'.repeat(64)), 'wrong hash stale')

// Frontmatter-less multi-slide still parses
const bare = `# A\n\n---\n\n# B\n`
const bareParsed = parseSlidesMarkdown(bare)
assert(bareParsed.pages.length === 2, 'bare pages')

const themed = `---
type: slides
theme: light
---

<!-- _class: lead -->
# Title

---

<!-- class: columns -->
## Left

## Right
`
const themedParsed = parseSlidesMarkdown(themed)
assert(themedParsed.theme === 'light', `theme=${themedParsed.theme}`)
assert(themedParsed.pages[0].layoutClass.includes('lead'), 'lead layout')
assert(themedParsed.pages[1].layoutClass.includes('columns'), 'columns layout')
const themedHtml = renderPlayableSlidesHtml(themed, await hashSlidesSource(themed))
assert(themedHtml.includes('data-theme="light"'), 'theme attr')
assert(themedHtml.includes('lead'), 'lead class in html')
assert(themedHtml.includes('columns'), 'columns class in html')

console.log('test-slides-render: ok')
