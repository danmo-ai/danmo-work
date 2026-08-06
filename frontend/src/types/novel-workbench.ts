export interface NovelFileNode {
  name: string
  path: string
  isDir: boolean
  size?: number
}

export interface NovelStateSummary {
  title: string
  stage: string
  lastCommittedCh: number
  nextAction: string
}

export type NovelStageAction = 'init' | 'outline' | 'write' | 'continue' | 'review'

export interface NovelStagePrefillCtx {
  bookId?: string
  chapter?: number
  chapterPath?: string
}

/** Parse a few scalar fields from novel-state.yaml without a YAML dependency. */
export function parseNovelStateYaml(raw: string): NovelStateSummary {
  const pick = (key: string): string => {
    const re = new RegExp(`^${key}:\\s*(.*)$`, 'm')
    const m = raw.match(re)
    if (!m) return ''
    let v = (m[1] ?? '').trim()
    if ((v.startsWith('"') && v.endsWith('"')) || (v.startsWith("'") && v.endsWith("'"))) {
      v = v.slice(1, -1)
    }
    if (v === '""' || v === "''") return ''
    return v
  }
  const last = Number.parseInt(pick('last_committed_ch'), 10)
  return {
    title: pick('title'),
    stage: pick('stage'),
    lastCommittedCh: Number.isFinite(last) ? last : 0,
    nextAction: pick('next_action'),
  }
}

export function novelBookDir(bookId: string): string {
  return `novel/${bookId}`
}

export function novelChaptersDir(bookId: string): string {
  return `novel/${bookId}/chapters`
}

export function novelStatePath(bookId: string): string {
  return `novel/${bookId}/novel-state.yaml`
}

export function novelBiblePath(bookId: string): string {
  return `novel/${bookId}/book-bible.md`
}

/** Sort chapter filenames like ch001.md, ch10.md naturally. */
export function sortChapterNodes(nodes: NovelFileNode[]): NovelFileNode[] {
  const files = nodes.filter((n) => !n.isDir && /\.md$/i.test(n.name))
  return files.slice().sort((a, b) => {
    const na = chapterNumFromName(a.name)
    const nb = chapterNumFromName(b.name)
    if (na != null && nb != null && na !== nb) return na - nb
    return a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: 'base' })
  })
}

export function chapterNumFromName(name: string): number | null {
  const m = name.match(/(\d+)/)
  if (!m) return null
  const n = Number.parseInt(m[1], 10)
  return Number.isFinite(n) ? n : null
}

export function nextChapterNumber(lastCommitted: number, chapterFiles: NovelFileNode[]): number {
  let maxFile = 0
  for (const f of chapterFiles) {
    const n = chapterNumFromName(f.name)
    if (n != null && n > maxFile) maxFile = n
  }
  return Math.max(lastCommitted, maxFile) + 1
}

export function buildNovelStagePrefill(action: NovelStageAction, ctx: NovelStagePrefillCtx): string {
  const bookId = (ctx.bookId ?? '').trim() || '<book-id>'
  const ch = ctx.chapter && ctx.chapter > 0 ? ctx.chapter : 0
  const chPad = ch > 0 ? String(ch).padStart(3, '0') : 'NNN'
  const chPath = ctx.chapterPath || `novel/${bookId}/chapters/ch${chPad}.md`

  switch (action) {
    case 'init':
      return [
        '开一本新书并立项：',
        '- 先用 ask_user 澄清题材、读者承诺、篇幅/平台、禁忌（一次一问即可）。',
        '- 创建 novel/<book-id>/（含 novel-state.yaml、book-bible.md、canon/、outline/、chapters/、reviews/、continuity/）。',
        '- 角色先 status=candidate，经我确认后再变 canon；确认前不要写正文。',
        '- 落盘后更新 novel-state（stage=init 或 outline）。',
      ].join('\n')
    case 'outline':
      return [
        `书目录：novel/${bookId}/`,
        '基于现有 book-bible / canon，写出：一句话梗概 + 分卷大纲 + 前若干章一句话章纲（含钩子）。',
        '先不要写章节正文；大纲写完停下来等我确认。',
      ].join('\n')
    case 'write':
      return [
        `写第 ${ch || 'N'} 章到 ${chPath}。`,
        '流程：章合同 → 草稿 → 审一轮 → Commit（更新伏笔表、timeline、章摘要与 novel-state）。',
        'P0 审稿不过不得定稿；不要只在对话里贴正文。',
      ].join('\n')
    case 'continue':
      return [
        `接着写下一章。先读 novel/${bookId}/novel-state.yaml 与近 3 章摘要、未回收伏笔，补章合同后写正文；`,
        '审一轮，P0 不过不得定稿；Commit 落盘并更新 novel-state。不要只在对话里贴正文。',
      ].join('\n')
    case 'review':
      return [
        `审阅 ${chPath}：按六透镜 + 去 AI 味 P0/P1 出审查报告，写入 novel/${bookId}/reviews/，`,
        'blocking 记入 continuity_issues；需要修订则直接改章节文件后再 Commit。',
      ].join('\n')
    default:
      return ''
  }
}
