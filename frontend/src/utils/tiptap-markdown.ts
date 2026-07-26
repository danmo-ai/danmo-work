import type { Editor, JSONContent } from '@tiptap/core'

/** Normalize editor markdown for disk / API (trailing newline). */
export function normalizeMarkdown(md: string): string {
  if (!md) return ''
  return md.replace(/\s+$/, '') + '\n'
}

/** Full document as markdown. */
export function editorToMarkdown(editor: Editor): string {
  return normalizeMarkdown(editor.getMarkdown())
}

/**
 * Serialize the current selection (or whole doc if empty) to markdown,
 * preserving marks/links unlike textBetween.
 */
export function selectionToMarkdown(editor: Editor): string {
  const { from, to, empty } = editor.state.selection
  if (empty) return editorToMarkdown(editor)

  const markdown = editor.markdown
  if (!markdown) return editorToMarkdown(editor)

  const slice = editor.state.doc.slice(from, to)
  const content = (slice.content.toJSON() as JSONContent[] | undefined) || []
  if (!content.length) return editorToMarkdown(editor)

  return normalizeMarkdown(markdown.serialize({ type: 'doc', content }))
}
