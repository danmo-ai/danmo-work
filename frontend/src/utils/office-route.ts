export type OfficeKind = 'doc' | 'slides' | 'sheet' | 'preview' | 'code' | 'diff'
export type OfficeMode = 'view' | 'edit' | 'present'
/** What the AI turn will target given current UI selection. */
export type OfficeEditScope = 'selection' | 'document' | 'slide' | 'sheet'

export interface OfficeRoute {
  kind: OfficeKind
  path: string
  mode: OfficeMode
  /** For preview kind: initial URL (project raw or proxied external). */
  url?: string
  /** For diff kind: show staged vs unstaged patch. */
  staged?: boolean
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

/** Common source / config extensions opened in the lightweight Code Surface. */
const CODE_EXT_RE =
  /\.(go|rs|py|js|jsx|mjs|cjs|ts|tsx|mts|cts|vue|svelte|java|kt|kts|c|cc|cpp|cxx|h|hpp|hxx|cs|swift|rb|php|lua|r|jl|zig|scala|clj|ex|exs|hs|ml|mli|sh|bash|zsh|fish|ps1|bat|cmd|sql|graphql|gql|proto|thrift|toml|ya?ml|json|jsonc|json5|xml|svg|css|scss|less|sass|html?|xhtml|ini|cfg|conf|env|properties|gradle|cmake|makefile|mk|dockerfile|tf|hcl|nix|lock|txt|log|gitignore|gitattributes|editorconfig|eslintrc|prettierrc|babelrc)$/i

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
    // Generic HTML stays preview (annotate/iframe); not Code Surface.
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

  if (isCodeFilePath(path)) {
    return { kind: 'code', path, mode: 'view' }
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
    '约束: 使用 read_file + edit/write 更新上述 path；不要改无关文件；幻灯片不要手写/覆盖 playable HTML（Stage 会从 md 程序同步）；完成后在 SUMMARY 写明变更位置。',
  )
  return lines.join('\n')
}
