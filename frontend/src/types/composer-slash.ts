export interface ComposerSlashCommand {
  id: string
  /** Trigger without leading slash, e.g. "changes". */
  trigger: string
  /** i18n key under composer.slash.* */
  labelKey: string
  descriptionKey: string
  keywords?: string[]
}

export const COMPOSER_SLASH_COMMANDS: ComposerSlashCommand[] = [
  {
    id: 'changes',
    trigger: 'changes',
    labelKey: 'composer.slash.changes',
    descriptionKey: 'composer.slash.changesDesc',
    keywords: ['diff', 'git', '变更'],
  },
  {
    id: 'plan',
    trigger: 'plan',
    labelKey: 'composer.slash.plan',
    descriptionKey: 'composer.slash.planDesc',
    keywords: ['计划'],
  },
  {
    id: 'files',
    trigger: 'files',
    labelKey: 'composer.slash.files',
    descriptionKey: 'composer.slash.filesDesc',
    keywords: ['文件'],
  },
  {
    id: 'memory',
    trigger: 'memory',
    labelKey: 'composer.slash.memory',
    descriptionKey: 'composer.slash.memoryDesc',
    keywords: ['记忆'],
  },
  {
    id: 'tables',
    trigger: 'tables',
    labelKey: 'composer.slash.tables',
    descriptionKey: 'composer.slash.tablesDesc',
    keywords: ['table', 'store', '表格', '表存储', '数据表'],
  },
  {
    id: 'terminal',
    trigger: 'terminal',
    labelKey: 'composer.slash.terminal',
    descriptionKey: 'composer.slash.terminalDesc',
    keywords: ['shell', '终端'],
  },
  {
    id: 'approve',
    trigger: 'approve',
    labelKey: 'composer.slash.approve',
    descriptionKey: 'composer.slash.approveDesc',
    keywords: ['pending', 'approval', 'ask', '审批'],
  },
  {
    id: 'stop',
    trigger: 'stop',
    labelKey: 'composer.slash.stop',
    descriptionKey: 'composer.slash.stopDesc',
    keywords: ['cancel', '停止'],
  },
  {
    id: 'new',
    trigger: 'new',
    labelKey: 'composer.slash.new',
    descriptionKey: 'composer.slash.newDesc',
    keywords: ['compose', '新会话'],
  },
]

/** Detect `/query` immediately before the caret (start of line or after whitespace). */
export function detectSlashQuery(
  text: string,
  caret: number,
): { start: number; query: string } | null {
  if (caret < 0 || caret > text.length) return null
  const before = text.slice(0, caret)
  const m = before.match(/(^|[\s\n])\/([^\s/]*)$/)
  if (!m) return null
  const query = m[2] ?? ''
  const start = before.length - query.length - 1
  if (start < 0 || text[start] !== '/') return null
  return { start, query }
}

export function removeSlashQuery(text: string, start: number, caret: number): string {
  return text.slice(0, start) + text.slice(caret)
}

export function filterSlashCommands(
  commands: ComposerSlashCommand[],
  query: string,
): ComposerSlashCommand[] {
  const q = query.trim().toLowerCase()
  if (!q) return commands
  return commands.filter((c) => {
    const hay = [c.trigger, c.id, ...(c.keywords ?? [])].join('\n').toLowerCase()
    return hay.includes(q) || c.trigger.startsWith(q)
  })
}
