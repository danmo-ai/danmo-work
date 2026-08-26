/**
 * Project file → Stage routing.
 *
 * Vocabulary:
 * - FileKind  — which Stage surface family (doc / slides / sheet / code / web / media / diff)
 * - FileEngine — persistence / editor backend (md, csv, univer-*, ms-office, codemirror, iframe, …)
 * - FileMode  — view | edit | present
 * - FileRoute — { kind, engine, mode, path, … } from routeProjectFile()
 *
 * Source code and web pages are first-class routes, same as Office IR files.
 */

export type FileKind = 'doc' | 'slides' | 'sheet' | 'code' | 'web' | 'media' | 'diff'
export type FileMode = 'view' | 'edit' | 'present'

/** Persistence / editor engine for the Stage surface. */
export type FileEngine =
  | 'md'
  | 'csv'
  | 'univer-sheet'
  | 'univer-doc'
  | 'univer-slides'
  | 'ms-office'
  | 'codemirror'
  | 'iframe'
  | 'image'
  | 'diff'

/** What an AI office-edit turn will target given current UI selection. */
export type FileEditScope = 'selection' | 'document' | 'slide' | 'sheet'

/** Kinds that support in-Stage AI polish/modify chips. */
export type EditableFileKind = Exclude<FileKind, 'code' | 'web' | 'media' | 'diff'>

export interface FileRoute {
  kind: FileKind
  path: string
  mode: FileMode
  engine: FileEngine
  /** For web kind: initial URL (project raw or proxied external). */
  url?: string
  /** For diff kind: show staged vs unstaged patch. */
  staged?: boolean
}

/** Common source / config extensions → Code Surface (CodeMirror). */
const CODE_EXT_RE =
  /\.(go|rs|py|js|jsx|mjs|cjs|ts|tsx|mts|cts|vue|svelte|java|kt|kts|c|cc|cpp|cxx|h|hpp|hxx|cs|swift|rb|php|lua|r|jl|zig|scala|clj|ex|exs|hs|ml|mli|sh|bash|zsh|fish|ps1|bat|cmd|sql|graphql|gql|proto|thrift|toml|ya?ml|json|jsonc|json5|xml|css|scss|less|sass|ini|cfg|conf|env|properties|gradle|cmake|makefile|mk|dockerfile|tf|hcl|nix|lock|txt|log|gitignore|gitattributes|editorconfig|eslintrc|prettierrc|babelrc)$/i

/** Raster / vector images → Media Surface. */
const IMAGE_EXT_RE = /\.(png|jpe?g|gif|webp|svg|ico|bmp|avif)$/i

const CODE_BASENAME_RE =
  /^(makefile|dockerfile|gemfile|rakefile|procfile|vagrantfile|jenkinsfile|\.gitignore|\.gitattributes|\.editorconfig|\.env|\.env\..+)$/i

export function isCodeFilePath(path: string): boolean {
  const lower = path.replace(/\\/g, '/').toLowerCase()
  const base = lower.split('/').pop() || lower
  if (CODE_BASENAME_RE.test(base)) return true
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
    case 'html':
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

export function isWebFilePath(path: string): boolean {
  const lower = path.replace(/\\/g, '/').toLowerCase()
  return lower.endsWith('.html') || lower.endsWith('.htm') || lower.endsWith('.xhtml')
}

export function isMediaFilePath(path: string): boolean {
  const lower = path.replace(/\\/g, '/').toLowerCase()
  const base = lower.split('/').pop() || lower
  return IMAGE_EXT_RE.test(base)
}

/**
 * Route a project-relative file path into a Stage surface.
 * Always returns a route. Source code and web pages are first-class.
 */
export function routeProjectFile(path: string, _contentHint?: string): FileRoute {
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

  // Web pages (HTML) — iframe Stage with design-mode annotate.
  if (isWebFilePath(path)) {
    return { kind: 'web', path, mode: 'view', engine: 'iframe' }
  }

  // Images / media — img preview Stage.
  if (isMediaFilePath(path)) {
    return { kind: 'media', path, mode: 'view', engine: 'image' }
  }

  // Markdown is always a document — never slides.
  if (base.endsWith('.md') || base.endsWith('.markdown')) {
    return { kind: 'doc', path, mode: 'view', engine: 'md' }
  }

  if (isCodeFilePath(path)) {
    return { kind: 'code', path, mode: 'view', engine: 'codemirror' }
  }

  // Unknown binary / other → media-like open via iframe/raw when possible.
  return { kind: 'web', path, mode: 'view', engine: 'iframe' }
}

function fileEditAsk(
  action: 'polish' | 'modify' | 'continue' | 'slide-page' | 'sheet',
  scope: FileEditScope | undefined,
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
 * Compact [office-edit] turn body for Composer (doc / slides / sheet only).
 * Keep header fields for routing / AI-review snapshots; body is locate + ask only.
 */
export function buildOfficeEditPrompt(opts: {
  action: 'polish' | 'modify' | 'continue' | 'slide-page' | 'sheet'
  path: string
  kind: FileKind
  selection: string
  instruction?: string
  pageIndex?: number
  scope?: FileEditScope
  startLine?: number
  endLine?: number
  review?: 'commit' | 'propose'
  engine?: FileEngine
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
  if (opts.review && opts.review !== 'commit') lines.push(`review: ${opts.review}`)

  lines.push('')
  lines.push(fileEditAsk(opts.action, opts.scope, opts.instruction))

  const sel = opts.selection.trimEnd()
  if (sel && sel !== '(cursor / end of document)') {
    lines.push('')
    lines.push('<<<')
    lines.push(sel)
    lines.push('>>>')
  }
  return lines.join('\n')
}
