import {
  buildOfficeEditPrompt,
  type OfficeEditScope,
  type OfficeKind,
} from '@/utils/office-route'

function createOfficeEditAttachmentId(): string {
  return `office_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 8)}`
}

export type OfficeEditAction = 'polish' | 'modify' | 'continue' | 'slide-page' | 'sheet'

/** Composer chip payload for office polish/modify (serializes to [office-edit] text on send). */
export interface OfficeEditAttachment {
  id: string
  kind: 'office-edit'
  action: OfficeEditAction
  path: string
  officeKind: Exclude<OfficeKind, 'preview' | 'code' | 'diff'>
  scope: OfficeEditScope
  /** User annotation / instruction (editable on chip). */
  instruction: string
  selection: string
  pageIndex?: number
  startLine?: number
  endLine?: number
}

export function createOfficeEditAttachment(opts: {
  action: OfficeEditAction
  path: string
  officeKind: OfficeEditAttachment['officeKind']
  scope: OfficeEditScope
  selection: string
  instruction?: string
  pageIndex?: number
  startLine?: number
  endLine?: number
}): OfficeEditAttachment {
  return {
    id: createOfficeEditAttachmentId(),
    kind: 'office-edit',
    action: opts.action,
    path: opts.path,
    officeKind: opts.officeKind,
    scope: opts.scope,
    instruction: (opts.instruction || '').trim(),
    selection: opts.selection,
    pageIndex: opts.pageIndex,
    startLine: opts.startLine,
    endLine: opts.endLine,
  }
}

function actionLabel(action: OfficeEditAction): string {
  switch (action) {
    case 'polish':
      return '润色'
    case 'continue':
      return '扩写'
    case 'slide-page':
      return '改页'
    case 'sheet':
      return '改表'
    case 'modify':
    default:
      return '修改'
  }
}

export function officeChipLabel(att: OfficeEditAttachment): string {
  const base = att.path.replace(/\\/g, '/').split('/').pop() || att.path
  const act = actionLabel(att.action)
  if (att.startLine != null && att.endLine != null) {
    if (att.startLine === att.endLine) return `${base}:${att.startLine} · ${act}`
    return `${base}:${att.startLine}–${att.endLine} · ${act}`
  }
  if (att.scope === 'slide' && att.pageIndex != null) return `${base} · p${att.pageIndex + 1} · ${act}`
  return `${base} · ${act}`
}

export function officeChipTooltip(att: OfficeEditAttachment): string {
  const lines: string[] = []
  lines.push(`动作: ${actionLabel(att.action)}`)
  if (att.instruction) lines.push(`批注: ${att.instruction}`)
  lines.push(`文件: ${att.path}`)
  lines.push(`范围: ${att.scope}`)
  if (att.startLine != null && att.endLine != null) {
    lines.push(
      att.startLine === att.endLine
        ? `行: ${att.startLine}`
        : `行: ${att.startLine}–${att.endLine}`,
    )
  }
  if (att.pageIndex != null) lines.push(`页: ${att.pageIndex + 1}`)
  const sel = att.selection.trim()
  if (sel && sel !== '(cursor / end of document)') {
    lines.push(`选区: ${sel.slice(0, 120)}${sel.length > 120 ? '…' : ''}`)
  }
  return lines.join('\n')
}

/** Wire format: plain [office-edit] text (not a binary file attachment). */
export function serializeOfficeEditAttachment(att: OfficeEditAttachment): string {
  return buildOfficeEditPrompt({
    action: att.action,
    path: att.path,
    kind: att.officeKind,
    selection: att.selection,
    instruction: att.instruction,
    pageIndex: att.pageIndex,
    scope: att.scope,
    startLine: att.startLine,
    endLine: att.endLine,
    review: 'commit',
  })
}

export function serializeOfficeEditAttachments(atts: OfficeEditAttachment[]): string {
  if (!atts.length) return ''
  return atts.map(serializeOfficeEditAttachment).join('\n\n')
}

export function officeEditSnapshotPaths(atts: OfficeEditAttachment[]): string[] {
  const out: string[] = []
  const seen = new Set<string>()
  for (const a of atts) {
    const p = a.path.trim()
    if (!p || seen.has(p)) continue
    seen.add(p)
    out.push(p)
  }
  return out
}
