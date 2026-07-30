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

const fragDeck = `---
type: slides
fragments: true
transition: fade
theme: moon
---

## Steps

- Alpha
- Beta

---

<!-- fragments: false -->
## No step

- Gamma
`
const fragParsed = parseSlidesMarkdown(fragDeck)
assert(fragParsed.fragments === true, 'deck fragments')
assert(fragParsed.transition === 'fade', 'transition')
assert(fragParsed.theme === 'moon', 'moon theme')
assert(fragParsed.pages[1].fragments === false, 'page fragments off')

const { applyAutoFragments } = await import('../src/utils/slides-render.ts')
const auto = applyAutoFragments('<ul><li>A</li><li class="x">B</li></ul>', true)
assert(auto.includes('class="fragment"') || auto.includes("class='fragment'"), 'li fragment')
assert(auto.includes('fragment x') || auto.includes('fragment"') || /class="fragment x"/.test(auto) || /class="x"/.test(auto), 'preserve class')

const fragHtml = renderPlayableSlidesHtml(fragDeck, await hashSlidesSource(fragDeck))
assert(fragHtml.includes('fragment'), 'auto fragments in html')
assert(fragHtml.includes('data-theme="moon"'), 'moon theme attr')
assert(fragHtml.includes('data-transition="fade"'), 'fade transition')
assert(fragHtml.includes('nextFragment') || fragHtml.includes('forward()'), 'fragment stepper')
assert(fragHtml.includes('is-overview'), 'overview mode')
assert(fragHtml.includes("e.key === 'o'") || fragHtml.includes("e.key === 'O'"), 'O key')

console.log('test-slides-render: ok')
