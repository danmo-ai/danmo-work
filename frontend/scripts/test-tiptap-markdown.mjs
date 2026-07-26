/**
 * Round-trip fixtures for TipTap v3 + @tiptap/markdown (DocSurface codec).
 * Run: npm run test:markdown
 */
import { Editor } from '@tiptap/core'
import StarterKit from '@tiptap/starter-kit'
import { Markdown } from '@tiptap/markdown'
import { normalizeMarkdown, selectionToMarkdown } from '../src/utils/tiptap-markdown.ts'

function createEditor(content = '') {
  return new Editor({
    element: null,
    extensions: [
      StarterKit.configure({
        link: { openOnClick: false },
      }),
      Markdown,
    ],
    content,
    contentType: 'markdown',
  })
}

function roundTrip(md) {
  const editor = createEditor(md)
  const out = normalizeMarkdown(editor.getMarkdown())
  editor.destroy()
  return out
}

function assert(cond, msg) {
  if (!cond) throw new Error(msg)
}

function assertIncludes(hay, needle, label) {
  assert(hay.includes(needle), `${label}: expected to include ${JSON.stringify(needle)}\n---\n${hay}`)
}

const fixtures = [
  {
    name: 'heading + paragraph + marks + link',
    input: `# Hello

This is **bold** and *italic* and a [link](https://example.com).
`,
    mustInclude: ['# Hello', '**bold**', '*italic*', '[link](https://example.com)'],
  },
  {
    name: 'lists',
    input: `- one
- two

1. first
2. second
`,
    mustInclude: ['- one', '- two', '1. first', '2. second'],
  },
  {
    name: 'fenced code',
    input: `\`\`\`js
console.log(1)
\`\`\`
`,
    mustInclude: ['```js', 'console.log(1)', '```'],
  },
  {
    name: 'blockquote',
    input: `> quoted line
`,
    mustInclude: ['>'],
  },
]

let failed = 0
for (const fx of fixtures) {
  try {
    const out = roundTrip(fx.input)
    for (const needle of fx.mustInclude) {
      assertIncludes(out, needle, fx.name)
    }
    console.log(`ok  ${fx.name}`)
  } catch (e) {
    failed += 1
    console.error(`fail ${fx.name}:`, e instanceof Error ? e.message : e)
  }
}

try {
  assert(normalizeMarkdown('hi') === 'hi\n', 'normalizeMarkdown trailing newline')
  assert(normalizeMarkdown('hi\n\n') === 'hi\n', 'normalizeMarkdown trim trailing')
  console.log('ok  normalizeMarkdown')
} catch (e) {
  failed += 1
  console.error('fail normalizeMarkdown:', e instanceof Error ? e.message : e)
}

try {
  const editor = createEditor(`This is **bold** text.\n`)
  let from = -1
  let to = -1
  editor.state.doc.descendants((node, pos) => {
    if (node.isText && node.text?.includes('bold')) {
      const i = node.text.indexOf('bold')
      from = pos + i
      to = from + 4
      return false
    }
  })
  editor.commands.setTextSelection({ from, to })
  const sel = selectionToMarkdown(editor)
  assertIncludes(sel, '**bold**', 'selectionToMarkdown')
  editor.destroy()
  console.log('ok  selectionToMarkdown preserves bold')
} catch (e) {
  failed += 1
  console.error('fail selectionToMarkdown:', e instanceof Error ? e.message : e)
}

if (failed) {
  console.error(`\n${failed} failure(s)`)
  process.exit(1)
}
console.log('\nall markdown codec checks passed')
