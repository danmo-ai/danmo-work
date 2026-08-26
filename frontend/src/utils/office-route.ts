export type OfficeKind = 'doc' | 'slides' | 'sheet' | 'preview' | 'code' | 'diff'
export type OfficeMode = 'view' | 'edit' | 'present'
/** Persistence / editor engine for the Stage surface. */
export type OfficeEngine =
  | 'md'
  | 'csv'
  | 'univer-sheet'
  | 'univer-doc'
  | 'univer-slides'
  | 'ms-office'
  | 'code'
  | 'preview'
  | 'diff'
/** What the AI turn will target given current UI selection. */
export type OfficeEditScope = 'selection' | 'document' | 'slide' | 'sheet'

export interface OfficeRoute {
  kind: OfficeKind
  path: string
  mode: OfficeMode
  engine: OfficeEngine
  /** For preview kind: initial URL (project raw or proxied external). */
  url?: string
  /** For diff kind: show staged vs unstaged patch. */
  staged?: boolean
}

/** Common source / config extensions opened in the lightweight Code Surface. */
const CODE_EXT_RE =
  /\.(go|rs|py|js|jsx|mjs|cjs|ts|tsx|mts|cts|vue|svelte|java|kt|kts|c|cc|cpp|cxx|h|hpp|hxx|cs|swift|rb|php|lua|r|jl|zig|scala|clj|ex|exs|hs|ml|mli|sh|bash|zsh|fish|ps1|bat|cmd|sql|graphql|gql|proto|thrift|toml|ya?ml|json|jsonc|json5|xml|css|scss|less|sass|html?|xhtml|ini|cfg|conf|env|properties|gradle|cmake|makefile|mk|dockerfile|tf|hcl|nix|lock|txt|log|gitignore|gitattributes|editorconfig|eslintrc|prettierrc|babelrc)$/i

/** Raster / vector images — Stage Preview (not Code Surface). */
const IMAGE_EXT_RE = /\.(png|jpe?g|gif|webp|svg|ico|bmp|avif)$/i

const CODE_BASENAME_RE =
  /^(makefile|dockerfile|gemfile|rakefile|procfile|vagrantfile|jenkinsfile|\.gitignore|\.gitattributes|\.editorconfig|\.env|\.env\..+)$/i

export function isCodeFilePath(path: string): boolean {
  const lower = path.replace(/\\/g, '/').toLowerCase()
  const base = lower.split('/').pop() || lower
  if (CODE_BASENAME_RE.test(base)) return true
  // Strip trailing dotted config names like .eslintrc.cjs already covered by ext.
  return CODE_EXT_RE.test(base)
}

export function languageFromPath(path: string): string {
  const base = (path.replace(/\\/g, '/').split('/').pop() || '').toLowerCase()
  if (CODE_BASENAME_RE.test(base) || base === 'makefile' || base.startsWith('makefile.')) return 'makefile'
  if (base === 'dockerfile' || base.startsWith('dockerfile.')) return 'dockerfile'
  const m = base.match(/\.([^.]+)$/)
  const ext = m?.[1] || ''
  switch (ext) {
    case 'ts':
    case 'tsx':
    case 'mts':
    case 'cts':
      return 'typescript'
    case 'js':
    case 'jsx':
    case 'mjs':
    case 'cjs':
      return 'javascript'
    case 'py':
      return 'python'
    case 'rs':
      return 'rust'
    case 'go':
      return 'go'
    case 'yml':
      return 'yaml'
    case 'md':
    case 'markdown':
      return 'markdown'
    case 'sh':
    case 'bash':
    case 'zsh':
    case 'fish':
      return 'shell'
    case 'htm':
      return 'html'
    default:
      return ext || 'text'
  }
}

export function isUniverSheetPath(path: string): boolean {
  return path.replace(/\\/g, '/').toLowerCase().endsWith('.usheet.json')
}

export function isUniverDocPath(path: string): boolean {
  return path.replace(/\\/g, '/').toLowerCase().endsWith('.udoc.json')
}

export function isUniverSlidesPath(path: string): boolean {
  return path.replace(/\\/g, '/').toLowerCase().endsWith('.uslides.json')
}

/** Legacy danmo sheet — no longer a SoT; migrate to .usheet.json on open. */
export function isLegacyDanmoSheetPath(path: string): boolean {
  return path.replace(/\\/g, '/').toLowerCase().endsWith('.danmo-sheet.json')
}

export function isMsOfficePath(path: string): boolean {
  const lower = path.replace(/\\/g, '/').toLowerCase()
  return lower.endsWith('.docx') || lower.endsWith('.xlsx') || lower.endsWith('.pptx')
}

/** Route a project-relative file path into a Stage kind. Always returns a route. */
export function routeOfficeFile(path: string, _contentHint?: string): OfficeRoute {
  const lower = path.replace(/\\/g, '/').toLowerCase()
  const base = lower.split('/').pop() || lower

  if (base.endsWith('.usheet.json')) {
    return { kind: 'sheet', path, mode: 'edit', engine: 'univer-sheet' }
  }
  if (base.endsWith('.udoc.json')) {
    return { kind: 'doc', path, mode: 'edit', engine: 'univer-doc' }
  }
  if (base.endsWith('.uslides.json')) {
    return { kind: 'slides', path, mode: 'edit', engine: 'univer-slides' }
  }

  // Legacy danmo-sheet: open as sheet view; Stage migrates to .usheet.json.
  if (base.endsWith('.danmo-sheet.json')) {
    return { kind: 'sheet', path, mode: 'view', engine: 'univer-sheet' }
  }

  if (base.endsWith('.csv')) {
    return { kind: 'sheet', path, mode: 'edit', engine: 'csv' }
  }

  if (base.endsWith('.xlsx')) {
    return { kind: 'sheet', path, mode: 'view', engine: 'ms-office' }
  }
  if (base.endsWith('.docx')) {
    return { kind: 'doc', path, mode: 'view', engine: 'ms-office' }
  }
  if (base.endsWith('.pptx')) {
    return { kind: 'slides', path, mode: 'view', engine: 'ms-office' }
  }

  // Legacy playable HTML decks: preview only (no MD slides SoT).
  if (base.endsWith('-slides.html') || (/(?:^|\/)slides?\//.test(lower) && base.endsWith('.html'))) {
    return { kind: 'preview', path, mode: 'view', engine: 'preview' }
  }

  if (base.endsWith('.html') || base.endsWith('.htm')) {
    return { kind: 'preview', path, mode: 'view', engine: 'preview' }
  }

  if (IMAGE_EXT_RE.test(base)) {
    return { kind: 'preview', path, mode: 'view', engine: 'preview' }
  }

  // Markdown is always a document — never slides.
  if (base.endsWith('.md') || base.endsWith('.markdown')) {
    return { kind: 'doc', path, mode: 'view', engine: 'md' }
  }

  if (isCodeFilePath(path)) {
    return { kind: 'code', path, mode: 'view', engine: 'code' }
  }

  return { kind: 'preview', path, mode: 'view', engine: 'preview' }
}

/** Short human ask for Composer — skills already teach tools / min-diff rules. */
function officeEditAsk(
  action: 'polish' | 'modify' | 'continue' | 'slide-page' | 'sheet',
  scope: OfficeEditScope | undefined,
  instruction?: string,
): string {
  const note = instruction?.trim()
  if (note) return note
  switch (action) {
    case 'polish':
      return scope === 'document' ? '请润色全文，保持原意，最小改动。' : '请润色选区，保持原意，最小改动。'
    case 'continue':
      return scope === 'document' ? '请续写全文末尾。' : '请扩写选区，丰富内容，保持原意。'
    case 'slide-page':
      return '请改进当前幻灯片页。'
    case 'sheet':
      return scope === 'selection' ? '请按选区修改表格。' : '请改进整表。'
    case 'modify':
    default:
      if (scope === 'slide') return '请修改当前幻灯片页。'
      if (scope === 'sheet') return '请修改表格。'
      if (scope === 'document') return '请修改全文。'
      return '请修改选区。'
  }
}

/**
 * Compact [office-edit] turn body for Composer.
 * Keep header fields for routing / AI-review snapshots; body is locate + ask only.
 */
export function buildOfficeEditPrompt(opts: {
  action: 'polish' | 'modify' | 'continue' | 'slide-page' | 'sheet'
  path: string
  kind: OfficeKind
  selection: string
  instruction?: string
  pageIndex?: number
  scope?: OfficeEditScope
  /** 1-based markdown line range for selection scope (doc). */
  startLine?: number
  endLine?: number
  /** commit = write SoT (default); propose = prefer minimal patch for human review */
  review?: 'commit' | 'propose'
  /** When set, agents must not write MS OOXML in place. */
  engine?: OfficeEngine
}): string {
  const lines = [
    '[office-edit]',
    `action: ${opts.action}`,
    `path: ${opts.path}`,
    `kind: ${opts.kind}`,
  ]
  if (opts.engine) lines.push(`engine: ${opts.engine}`)
  if (opts.scope) lines.push(`scope: ${opts.scope}`)
  if (opts.pageIndex != null) lines.push(`page: ${opts.pageIndex}`)
  if (
    opts.startLine != null &&
    opts.endLine != null &&
    opts.startLine > 0 &&
    opts.endLine >= opts.startLine
  ) {
    lines.push(
      opts.startLine === opts.endLine
        ? `lines: ${opts.startLine}`
        : `lines: ${opts.startLine}-${opts.endLine}`,
    )
  }
  // review kept for agents that branch on propose; omit default commit noise
  if (opts.review && opts.review !== 'commit') lines.push(`review: ${opts.review}`)

  lines.push('')
  lines.push(officeEditAsk(opts.action, opts.scope, opts.instruction))

  const sel = opts.selection.trimEnd()
  if (sel && sel !== '(cursor / end of document)') {
    lines.push('')
    lines.push('<<<')
    lines.push(sel)
    lines.push('>>>')
  }
  return lines.join('\n')
}
