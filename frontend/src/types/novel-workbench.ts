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

/** Full novel-writing skill pipeline (routes.md order). */
export type NovelStageAction =
  | 'init'
  | 'outline'
  | 'assets'
  | 'goldfinger'
  | 'contract'
  | 'write'
  | 'continue'
  | 'review'
  | 'polish'
  | 'commit'

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

export function novelContinuityDir(bookId: string): string {
  return `novel/${bookId}/continuity`
}

export function novelOutlineDir(bookId: string): string {
  return `novel/${bookId}/outline`
}

export function novelReviewsDir(bookId: string): string {
  return `novel/${bookId}/reviews`
}

export function novelCanonDir(bookId: string): string {
  return `novel/${bookId}/canon`
}

export function novelChapterSummariesPath(bookId: string): string {
  return `novel/${bookId}/continuity/chapter_summaries.md`
}

export function novelChapterFilePath(bookId: string, chapter: number): string {
  const pad = String(chapter).padStart(3, '0')
  return `novel/${bookId}/chapters/ch${pad}.md`
}

export function isNovelChapterPath(path: string): boolean {
  return /\/chapters\/[^/]+\.md$/i.test(path.replace(/\\/g, '/'))
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

export function sortMdNodes(nodes: NovelFileNode[]): NovelFileNode[] {
  return nodes
    .filter((n) => !n.isDir && /\.md$/i.test(n.name))
    .slice()
    .sort((a, b) => a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: 'base' }))
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
  const root = `novel/${bookId}`

  switch (action) {
    case 'init':
      return [
        '开一本新书并立项：',
        '- 先用 ask_user 澄清题材、读者承诺、篇幅/平台、禁忌（一次一问即可）。',
        '- 创建 novel/<book-id>/（含 novel-state.yaml、book-bible.md、canon/、outline/、chapters/、reviews/、continuity/）。',
        '- 角色先 status=candidate，经我确认后再变 canon；确认前不要写正文。',
        '- 落盘后更新 novel-state（stage=init 或 outline）。',
        '- 按 read_skill novel-writing/references/init.md 与 project-layout.md 执行。',
      ].join('\n')
    case 'outline':
      return [
        `书目录：${root}/`,
        '基于现有 book-bible / canon，写出：一句话梗概 + 分卷大纲 + 前若干章一句话章纲（含钩子）。',
        '先不要写章节正文；大纲写完停下来等我确认。',
        '按 read_skill novel-writing/references/outline.md 执行。',
      ].join('\n')
    case 'assets':
      return [
        `书目录：${root}/`,
        '整理/补全人物卡与世界观资产：写入 canon/ 与 table_*（characters、locations 等，带 book_id）。',
        '新实体先 status=candidate，经 ask_user 确认后再变 canon；确认前不要写正文。',
        '按 read_skill novel-writing/references/init.md + table-schema.md 执行。',
      ].join('\n')
    case 'goldfinger':
      return [
        `书目录：${root}/`,
        '设计或修订金手指：约束、代价、成长曲线、与读者承诺一致；写入 canon/ 或 table_* resources，并用模板 goldfinger-card。',
        '先 candidate，经 ask_user 确认后再 canon；不要在未确认时改已定稿正文。',
        '按 read_skill novel-writing/assets/templates/goldfinger-card.md 与 KB「金手指约束」执行。',
      ].join('\n')
    case 'contract':
      return [
        `为第 ${ch || 'N'} 章写章合同（尚不写正文）。`,
        `落盘到 ${root}/outline/ 或 table_upsert chapter_contracts（book_id=${bookId}）。`,
        '含：目的、must_happen / must_not、进出状态、伏笔、章末钩子、连续性风险；status=proposed。',
        '里程碑章或本批首章需 ask_user 接受后再进入写作。',
        '按 read_skill novel-writing/references/chapter-contract.md 执行。',
      ].join('\n')
    case 'write':
      return [
        `写第 ${ch || 'N'} 章正文到 ${chPath}。`,
        '前提：该章合同已 accepted（否则先补合同）。',
        '流程：preflight → knowledge/asset 门 → 按合同草稿 → 不要跳过审稿与 Commit。',
        'P0 审稿不过不得定稿；不要只在对话里贴正文。',
        '按 read_skill novel-writing/references/chapter-write.md 执行。',
      ].join('\n')
    case 'continue':
      return [
        `接着写下一章（书：${root}/）。`,
        '先读 novel-state.yaml、近 3 章摘要、未回收伏笔与开放 continuity_issues；补/更新章合同后再写正文。',
        '审一轮，P0 不过不得定稿；Commit 落盘并更新 novel-state。不要只在对话里贴正文。',
        '按 read_skill novel-writing/references/chapter-write.md 与 continuity-commit.md 执行。',
      ].join('\n')
    case 'review':
      return [
        `审阅 ${chPath}：按六透镜 + 去 AI 味 P0/P1 出审查报告，写入 ${root}/reviews/，`,
        'blocking 记入 continuity_issues；需要修订则直接改章节文件。',
        'qc_gate FAIL 不得宣称定稿。按 read_skill novel-writing/references/review-gates.md 执行。',
      ].join('\n')
    case 'polish':
      return [
        `对 ${chPath} 做去 AI 味润色（deslop）：先 knowledge_gate（去 AI 味 / 文风），再 edit 落盘。`,
        '先清 P0 再 P1；不要改情节 Canon；需要改情节则回到审稿/章合同。',
        '按 read_skill novel-writing/references/polish-deslop.md 执行。',
      ].join('\n')
    case 'commit':
      return [
        ch > 0
          ? `对第 ${ch} 章（${chPath}）做 Continuity Commit。`
          : `对当前进度做 Continuity Commit（书：${root}/）。`,
        '更新：章节定稿文件、table_*（timeline / foreshadows / characters / contracts）、continuity/chapter_summaries.md、memory、novel-state。',
        '关闭已修复的 continuity_issues。每满 10 个已提交章写 continuity/phase-NN.md。',
        '按 read_skill novel-writing/references/continuity-commit.md 执行。完成=工具证据，勿口头宣称。',
      ].join('\n')
    default:
      return ''
  }
}
