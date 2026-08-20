/** Parse user message text into plain + structured chips for stream display. */

export type UserMessagePart =
  | { type: 'text'; text: string }
  | {
      type: 'office-edit'
      action: string
      path: string
      scope?: string
      lines?: string
      page?: string
      ask: string
      selectionPreview: string
    }
  | {
      type: 'selected-code'
      path: string
      lines?: string
      request?: string
      preview: string
    }
  | {
      type: 'selected-element'
      summary: string
      request?: string
      preview: string
    }
  | {
      type: 'preview-console'
      request?: string
      preview: string
    }

const OFFICE_RE =
  /\[office-edit\]\n([\s\S]*?)(?=\n\[office-edit\]\n|\n## Selected (?:Code|UI Element)\n|$)/g
const CODE_RE =
  /## Selected Code\n([\s\S]*?)(?=\n\[office-edit\]\n|\n## Selected (?:Code|UI Element)\n|$)/g
const ELEMENT_RE =
  /## Selected UI Element\n([\s\S]*?)(?=\n\[office-edit\]\n|\n## Selected (?:Code|UI Element)\n|\n## Preview Console \/ Network\n|$)/g
const CONSOLE_RE =
  /## Preview Console \/ Network\n([\s\S]*?)(?=\n\[office-edit\]\n|\n## Selected (?:Code|UI Element)\n|\n## Preview Console \/ Network\n|$)/g

function field(block: string, key: string): string | undefined {
  const m = block.match(new RegExp(`^${key}:\\s*(.+)$`, 'm'))
  return m?.[1]?.trim()
}

function extractAskAndSelection(block: string): { ask: string; selectionPreview: string } {
  const fence = block.match(/<<<\n([\s\S]*?)\n>>>/)
  const selection = fence?.[1]?.trim() || ''
  const before = fence ? block.slice(0, fence.index) : block
  // Drop header key:value lines; keep the human ask line(s).
  const ask = before
    .split('\n')
    .filter((l) => l.trim() && !/^(action|path|kind|scope|page|lines|review):/i.test(l.trim()))
    .join('\n')
    .trim()
  const preview =
    selection.length > 160 ? `${selection.slice(0, 160)}…` : selection
  return { ask, selectionPreview: preview }
}

function parseOfficeBlock(raw: string): UserMessagePart {
  const { ask, selectionPreview } = extractAskAndSelection(raw)
  return {
    type: 'office-edit',
    action: field(raw, 'action') || 'modify',
    path: field(raw, 'path') || '',
    scope: field(raw, 'scope'),
    lines: field(raw, 'lines'),
    page: field(raw, 'page'),
    ask,
    selectionPreview,
  }
}

function parseCodeBlock(raw: string): UserMessagePart {
  const path = field(raw, 'File') || ''
  const lines = field(raw, 'Lines')
  const request = field(raw, 'Request')
  const code = raw.match(/```[^\n]*\n([\s\S]*?)```/)?.[1]?.trim() || ''
  const preview = code.length > 120 ? `${code.slice(0, 120)}…` : code
  return { type: 'selected-code', path, lines, request, preview }
}

function parseElementBlock(raw: string): UserMessagePart {
  const request = field(raw, 'Request') || field(raw, 'Annotation')
  const target = field(raw, 'Target') || field(raw, 'File') || 'element'
  const html = raw.match(/```html\n([\s\S]*?)```/)?.[1]?.trim() || ''
  const preview = html.length > 100 ? `${html.slice(0, 100)}…` : html
  return { type: 'selected-element', summary: target, request, preview }
}

function parseConsoleBlock(raw: string): UserMessagePart {
  const request = field(raw, 'Request')
  const preview = raw.replace(/^Request:.*$/m, '').trim().slice(0, 120)
  return { type: 'preview-console', request, preview }
}

type MatchHit = { index: number; length: number; part: UserMessagePart }

/** Split userInput into display parts; structured blocks become chips. */
export function parseUserMessageParts(text: string): UserMessagePart[] {
  if (!text) return []
  const hits: MatchHit[] = []

  for (const re of [OFFICE_RE, CODE_RE, ELEMENT_RE, CONSOLE_RE]) {
    re.lastIndex = 0
    let m: RegExpExecArray | null
    while ((m = re.exec(text)) !== null) {
      const body = m[1] ?? ''
      const full = m[0]
      let part: UserMessagePart
      if (full.startsWith('[office-edit]')) part = parseOfficeBlock(body)
      else if (full.startsWith('## Selected Code')) part = parseCodeBlock(body)
      else if (full.startsWith('## Preview Console')) part = parseConsoleBlock(body)
      else part = parseElementBlock(body)
      hits.push({ index: m.index, length: full.length, part })
    }
  }

  hits.sort((a, b) => a.index - b.index)

  const parts: UserMessagePart[] = []
  let cursor = 0
  for (const h of hits) {
    if (h.index < cursor) continue
    const gap = text.slice(cursor, h.index).trim()
    if (gap) parts.push({ type: 'text', text: gap })
    parts.push(h.part)
    cursor = h.index + h.length
  }
  const rest = text.slice(cursor).trim()
  if (rest) parts.push({ type: 'text', text: rest })
  if (!parts.length && text.trim()) parts.push({ type: 'text', text: text.trim() })
  return parts
}

export function officeActionLabel(action: string): string {
  switch (action) {
    case 'polish':
      return '润色'
    case 'continue':
      return '扩写'
    case 'slide-page':
      return '改页'
    case 'sheet':
      return '改表'
    default:
      return '修改'
  }
}
