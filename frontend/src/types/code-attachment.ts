/** Structured code selection attachment for Composer chips (line-range annotate). */

export interface CodeSelectionAttachment {
  id: string
  kind: 'code-selection'
  annotation: string
  path: string
  language: string
  startLine: number
  endLine: number
  /** Selected source text (may be truncated for prompt size). */
  text: string
}

const MAX_CODE_TEXT = 4000

export function createCodeSelectionAttachmentId(): string {
  return `code_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 8)}`
}

export function createCodeSelectionAttachment(opts: {
  path: string
  language: string
  startLine: number
  endLine: number
  text: string
  annotation?: string
}): CodeSelectionAttachment {
  const start = Math.max(1, Math.min(opts.startLine, opts.endLine))
  const end = Math.max(1, Math.max(opts.startLine, opts.endLine))
  let text = opts.text
  if (text.length > MAX_CODE_TEXT) {
    text = text.slice(0, MAX_CODE_TEXT) + '\n…'
  }
  return {
    id: createCodeSelectionAttachmentId(),
    kind: 'code-selection',
    annotation: (opts.annotation || '').trim(),
    path: opts.path,
    language: opts.language || 'text',
    startLine: start,
    endLine: end,
    text,
  }
}

export function codeChipLabel(att: CodeSelectionAttachment): string {
  const base = att.path.replace(/\\/g, '/').split('/').pop() || att.path
  if (att.startLine === att.endLine) return `${base}:${att.startLine}`
  return `${base}:${att.startLine}–${att.endLine}`
}

export function codeChipTooltip(att: CodeSelectionAttachment): string {
  const lines: string[] = []
  if (att.annotation) lines.push(`批注: ${att.annotation}`)
  lines.push(`文件: ${att.path}`)
  lines.push(
    att.startLine === att.endLine
      ? `行: ${att.startLine}`
      : `行: ${att.startLine}–${att.endLine}`,
  )
  if (att.language) lines.push(`语言: ${att.language}`)
  if (att.text) lines.push(`代码: ${att.text.slice(0, 120)}${att.text.length > 120 ? '…' : ''}`)
  return lines.join('\n')
}

/** Compact prompt block — path + line range + fenced code for the agent. */
export function serializeCodeSelectionAttachment(att: CodeSelectionAttachment): string {
  const lines: string[] = ['## Selected Code']
  if (att.annotation) lines.push(`Request: ${att.annotation}`)
  lines.push(`File: ${att.path}`)
  if (att.startLine === att.endLine) {
    lines.push(`Lines: ${att.startLine}`)
  } else {
    lines.push(`Lines: ${att.startLine}-${att.endLine}`)
  }
  if (att.language) lines.push(`Language: ${att.language}`)
  const body = att.text.trimEnd()
  if (body) {
    const fence = att.language && /^[a-z0-9_+-]+$/i.test(att.language) ? att.language : ''
    lines.push('Code:')
    lines.push('```' + fence)
    lines.push(body)
    lines.push('```')
  }
  lines.push(
    '约束: 优先用 read_file / edit 针对上述 File 与 Lines 范围修改；在 SUMMARY 中写明行号。',
  )
  return lines.join('\n')
}

export function serializeCodeSelectionAttachments(atts: CodeSelectionAttachment[]): string {
  if (!atts.length) return ''
  return atts.map(serializeCodeSelectionAttachment).join('\n\n')
}

/** 1-based line number for a character offset in text. */
export function lineNumberAtOffset(text: string, offset: number): number {
  const n = Math.max(0, Math.min(offset, text.length))
  let line = 1
  for (let i = 0; i < n; i++) {
    if (text.charCodeAt(i) === 10) line++
  }
  return line
}

/**
 * Line range for a textarea selection.
 * Empty selection → single line at caret.
 * End offset on a newline boundary counts the previous line as last selected.
 */
export function selectionLineRange(
  text: string,
  start: number,
  end: number,
): { startLine: number; endLine: number } {
  const a = Math.max(0, Math.min(start, end, text.length))
  const b = Math.max(0, Math.max(start, end), 0)
  const endAdj = b > a ? b - 1 : a
  return {
    startLine: lineNumberAtOffset(text, a),
    endLine: lineNumberAtOffset(text, Math.min(endAdj, text.length)),
  }
}
