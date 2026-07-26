export type OfficeKind = 'doc' | 'slides' | 'sheet' | 'preview'
export type OfficeMode = 'view' | 'edit' | 'present'
/** What the AI turn will target given current UI selection. */
export type OfficeEditScope = 'selection' | 'document' | 'slide' | 'sheet'

export interface OfficeRoute {
  kind: OfficeKind
  path: string
  mode: OfficeMode
  /** For preview kind: initial URL (project raw or proxied external). */
  url?: string
}

/** Detect slides markdown: YAML type: slides, or multiple --- slide separators. */
export function looksLikeSlidesMarkdown(content: string): boolean {
  const trimmed = content.trimStart()
  if (/^---\r?\n[\s\S]*?\btype\s*:\s*slides\b/i.test(trimmed)) return true
  // Horizontal rules used as slide breaks (not YAML frontmatter alone).
  const parts = content.split(/^\s*---\s*$/m)
  if (parts.length >= 3) {
    // Likely frontmatter + slides, or multiple slides.
    const body = parts.slice(1).join('\n')
    return body.trim().length > 0
  }
  return false
}

/** Route a project-relative file path into a Stage kind. Always returns a route. */
export function routeOfficeFile(path: string, contentHint?: string): OfficeRoute {
  const lower = path.replace(/\\/g, '/').toLowerCase()
  const base = lower.split('/').pop() || lower

  if (base.endsWith('.csv') || base.endsWith('.danmo-sheet.json')) {
    return { kind: 'sheet', path, mode: 'edit' }
  }

  if (base.endsWith('-slides.html') || (/(?:^|\/)slides?\//.test(lower) && base.endsWith('.html'))) {
    return { kind: 'slides', path, mode: 'present' }
  }

  if (base.endsWith('.html') || base.endsWith('.htm')) {
    // Playable decks often mention keyboard presentation; default preview for generic HTML.
    if (contentHint && /playable-slides|data-slide|slide-deck|class=["']slide/i.test(contentHint)) {
      return { kind: 'slides', path, mode: 'present' }
    }
    return { kind: 'preview', path, mode: 'view' }
  }

  if (base.endsWith('.md') || base.endsWith('.markdown')) {
    if (contentHint && looksLikeSlidesMarkdown(contentHint)) {
      return { kind: 'slides', path, mode: 'edit' }
    }
    // Filename hints without reading content.
    if (/-slides\.md$|\/slides\/|\/deck\//i.test(lower) || base.startsWith('slides.')) {
      return { kind: 'slides', path, mode: 'edit' }
    }
    return { kind: 'doc', path, mode: 'edit' }
  }

  return { kind: 'preview', path, mode: 'view' }
}

export function buildOfficeEditPrompt(opts: {
  action: 'polish' | 'modify' | 'continue' | 'slide-page' | 'sheet'
  path: string
  kind: OfficeKind
  selection: string
  instruction?: string
  pageIndex?: number
  scope?: OfficeEditScope
}): string {
  const lines = [
    '[office-edit]',
    `action: ${opts.action}`,
    `path: ${opts.path}`,
    `kind: ${opts.kind}`,
  ]
  if (opts.scope) lines.push(`scope: ${opts.scope}`)
  if (opts.pageIndex != null) lines.push(`page: ${opts.pageIndex}`)
  lines.push('selection:')
  lines.push('<<<')
  lines.push(opts.selection.trimEnd())
  lines.push('>>>')
  lines.push(`instruction: ${opts.instruction?.trim() || '(none)'}`)
  lines.push(
    '约束: 使用 read_file + edit/write 更新上述 path；不要改无关文件；不要输出整份 HTML 覆盖；完成后在 SUMMARY 写明变更位置。',
  )
  return lines.join('\n')
}
