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

export type GateStatus = 'unknown' | 'pass' | 'fail' | 'skipped'

export interface NovelExtendedState extends NovelStateSummary {
  qcProfile: string
  continuationMode: boolean
  frozenBatch: { from: number; to: number } | null
  batchFreezeArtifact: string
  gates: { knowledge: GateStatus; asset: GateStatus; qc: GateStatus }
  blockers: string[]
}

export type NovelPipelinePhase =
  | 'init'
  | 'setup'
  | 'outline'
  | 'batch_freeze'
  | 'chapter_loop'
  | 'continuation'
  | 'idle'

export type NovelChapterPhase =
  | 'empty'
  | 'contract_draft'
  | 'contract_ready'
  | 'drafted'
  | 'review_fail'
  | 'review_pass'
  | 'committed'

/** Full novel-writing skill pipeline (routes.md order). */
export type NovelStageAction =
  | 'init'
  | 'outline'
  | 'volume'
  | 'assets'
  | 'goldfinger'
  | 'contract'
  | 'write'
  | 'continue'
  | 'review'
  | 'polish'
  | 'commit'
  | 'batch-freeze'
  | 'continuation'
  | 'batch-review'
  | 'preflight'

export interface NovelStagePrefillCtx {
  bookId?: string
  chapter?: number
  chapterPath?: string
  volume?: number
  batchFrom?: number
  batchTo?: number
}

export interface NovelActionDecision {
  allowed: boolean
  blockers: string[]
}

export interface NovelBookPipeline {
  phase: NovelPipelinePhase
  primaryAction: NovelStageAction | null
  primaryChapter?: number
  gates: { knowledge: GateStatus; asset: GateStatus; qc: GateStatus }
  blockers: string[]
  progress: { committed: number; totalWithContract: number; percent: number }
  frozenBatch: { from: number; to: number } | null
}

export interface NovelBookContext {
  bookId: string
  state: NovelExtendedState
  entries: NovelChapterEntry[]
  chapterPhases: Record<number, NovelChapterPhase>
  castFileCount: number
  hasBookOutline: boolean
  hasVolumeOutline: boolean
  hasBatchFreezeFile: boolean
  batchFreezeFrozen: boolean
}

function yamlScalar(raw: string, key: string): string {
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

function yamlNestedScalar(raw: string, parent: string, key: string): string {
  const blockRe = new RegExp(`^${parent}:\\s*\\n([\\s\\S]*?)(?=^\\S|$)`, 'm')
  const block = raw.match(blockRe)
  if (!block) return ''
  const re = new RegExp(`^\\s+${key}:\\s*(.*)$`, 'm')
  const m = block[1].match(re)
  if (!m) return ''
  return (m[1] ?? '').trim().replace(/^["']|["']$/g, '')
}

/** Parse a few scalar fields from novel-state.yaml without a YAML dependency. */
export function parseNovelStateYaml(raw: string): NovelStateSummary {
  const last = Number.parseInt(yamlScalar(raw, 'last_committed_ch'), 10)
  return {
    title: yamlScalar(raw, 'title'),
    stage: yamlScalar(raw, 'stage'),
    lastCommittedCh: Number.isFinite(last) ? last : 0,
    nextAction: yamlScalar(raw, 'next_action'),
  }
}

export function parseNovelStateExtended(raw: string): NovelExtendedState {
  const base = parseNovelStateYaml(raw)
  const from = Number.parseInt(yamlNestedScalar(raw, 'frozen_batch', 'from'), 10)
  const to = Number.parseInt(yamlNestedScalar(raw, 'frozen_batch', 'to'), 10)
  let frozenBatch =
    Number.isFinite(from) && Number.isFinite(to) && from > 0 && to >= from
      ? { from, to }
      : null
  if (!frozenBatch) {
    const batchBlock = raw.match(/^frozen_batch:\s*\n((?:[ \t].*\n?)*)/m)
    if (batchBlock) {
      const f = batchBlock[1].match(/^\s+from:\s*(\d+)/m)
      const t = batchBlock[1].match(/^\s+to:\s*(\d+)/m)
      const fromN = f ? Number.parseInt(f[1], 10) : NaN
      const toN = t ? Number.parseInt(t[1], 10) : NaN
      if (Number.isFinite(fromN) && Number.isFinite(toN) && fromN > 0 && toN >= fromN) {
        frozenBatch = { from: fromN, to: toN }
      }
    }
  }

  const parseGateBlock = (rawYaml: string): { knowledge: GateStatus; asset: GateStatus; qc: GateStatus } => {
    const defaults: { knowledge: GateStatus; asset: GateStatus; qc: GateStatus } = {
      knowledge: 'unknown',
      asset: 'unknown',
      qc: 'unknown',
    }
    const block = rawYaml.match(/^gates:\s*\n((?:[ \t].*\n?)*)/m)
    if (!block) return defaults
    const parseOne = (k: string): GateStatus => {
      const m = block[1].match(new RegExp(`^\\s+${k}:\\s*(\\w+)`, 'm'))
      const v = (m?.[1] ?? '').toLowerCase()
      if (v === 'pass' || v === 'fail' || v === 'skipped') return v
      return 'unknown'
    }
    return {
      knowledge: parseOne('knowledge'),
      asset: parseOne('asset'),
      qc: parseOne('qc'),
    }
  }

  const artifactsBlock = raw.match(/^artifacts:\s*\n((?:[ \t].*\n?)*)/m)
  let batchFreezeArtifact = yamlNestedScalar(raw, 'artifacts', 'batch_freeze')
  if (!batchFreezeArtifact && artifactsBlock) {
    const m = artifactsBlock[1].match(/^\s+batch_freeze:\s*(.+)$/m)
    if (m) batchFreezeArtifact = m[1].trim().replace(/^["']|["']$/g, '')
  }

  const blockers: string[] = []
  const blockerBlock = raw.match(/^blockers:\s*\n((?:\s+-\s+.+\n?)*)/m)
  if (blockerBlock) {
    for (const line of blockerBlock[1].split('\n')) {
      const m = line.match(/^\s*-\s+(.*)$/)
      if (m?.[1]?.trim()) blockers.push(m[1].trim())
    }
  }

  return {
    ...base,
    qcProfile: yamlScalar(raw, 'qc_profile') || 'general',
    continuationMode: /continuation_mode:\s*true/i.test(raw),
    frozenBatch,
    batchFreezeArtifact,
    gates: parseGateBlock(raw),
    blockers,
  }
}

export function parseContractYaml(raw: string): { status: string } {
  const status = yamlScalar(raw, 'status').toLowerCase()
  return { status: status || 'proposed' }
}

export function parseReviewVerdict(raw: string): 'PASS' | 'FAIL' | null {
  const m = raw.match(/###\s*VERDICT\s*\n\s*(PASS|FAIL)/i)
  if (!m) return null
  return m[1].toUpperCase() === 'PASS' ? 'PASS' : 'FAIL'
}

export function parseBatchFreezeYaml(raw: string): { status: string } {
  return { status: yamlScalar(raw, 'status').toLowerCase() || 'proposed' }
}

export function inferChapterPhase(
  entry: NovelChapterEntry,
  lastCommittedCh: number,
  contractRaw?: string,
  reviewRaw?: string,
): NovelChapterPhase {
  if (entry.chapter <= lastCommittedCh && entry.prose) return 'committed'

  const contractStatus = contractRaw ? parseContractYaml(contractRaw).status : ''
  const verdict = reviewRaw ? parseReviewVerdict(reviewRaw) : null

  if (entry.prose) {
    if (verdict === 'FAIL') return 'review_fail'
    if (verdict === 'PASS') return 'review_pass'
    return 'drafted'
  }

  if (entry.contract) {
    if (contractStatus === 'accepted' || contractStatus === 'drafted' || contractStatus === 'reviewed') {
      return 'contract_ready'
    }
    return 'contract_draft'
  }

  return 'empty'
}

export function inferChapterNextAction(phase: NovelChapterPhase): NovelStageAction | null {
  switch (phase) {
    case 'empty':
      return 'contract'
    case 'contract_draft':
      return 'contract'
    case 'contract_ready':
      return 'write'
    case 'drafted':
    case 'review_fail':
      return 'review'
    case 'review_pass':
      return 'commit'
    case 'committed':
      return null
    default:
      return null
  }
}

export function buildChapterPhases(
  entries: NovelChapterEntry[],
  lastCommittedCh: number,
  contractRaws: Record<number, string> = {},
  reviewRaws: Record<number, string> = {},
): Record<number, NovelChapterPhase> {
  const out: Record<number, NovelChapterPhase> = {}
  for (const e of entries) {
    out[e.chapter] = inferChapterPhase(
      e,
      lastCommittedCh,
      contractRaws[e.chapter],
      reviewRaws[e.chapter],
    )
  }
  return out
}

export function inferBookPipelinePhase(ctx: NovelBookContext): NovelPipelinePhase {
  const { state, hasBookOutline, hasVolumeOutline, batchFreezeFrozen, continuationMode } = ctx
  if (continuationMode || state.stage === 'continuation') return 'continuation'
  if (state.stage === 'batch_freeze' || (hasVolumeOutline && !batchFreezeFrozen)) {
    return hasVolumeOutline && !batchFreezeFrozen ? 'batch_freeze' : 'chapter_loop'
  }
  if (state.stage === 'init' || !hasBookOutline) return hasBookOutline ? 'setup' : 'init'
  if (state.stage === 'outline' || !hasVolumeOutline) return 'outline'
  if (hasVolumeOutline && !batchFreezeFrozen && ctx.entries.some((e) => e.prose)) {
    return 'chapter_loop'
  }
  if (hasVolumeOutline && !batchFreezeFrozen) return 'batch_freeze'
  return 'chapter_loop'
}

export function computeBookPipeline(ctx: NovelBookContext): NovelBookPipeline {
  const phase = inferBookPipelinePhase(ctx)
  const committed = ctx.state.lastCommittedCh
  const totalWithContract = ctx.entries.filter((e) => e.contract).length
  const percent =
    totalWithContract > 0 ? Math.min(100, Math.round((committed / totalWithContract) * 100)) : 0

  const assetGate: GateStatus =
    ctx.castFileCount > 0 ? 'pass' : ctx.state.gates.asset === 'fail' ? 'fail' : 'unknown'

  const gates = {
    knowledge: ctx.state.gates.knowledge,
    asset: assetGate,
    qc: ctx.state.gates.qc,
  }

  const blockers = [...ctx.state.blockers]
  if (assetGate === 'unknown' && ctx.castFileCount === 0) {
    blockers.push('blocker.noCast')
  }

  let primaryAction: NovelStageAction | null = null
  let primaryChapter: number | undefined

  if (phase === 'init') primaryAction = 'init'
  else if (phase === 'setup') primaryAction = 'assets'
  else if (phase === 'outline') primaryAction = 'outline'
  else if (phase === 'batch_freeze') primaryAction = 'batch-freeze'
  else if (phase === 'continuation') primaryAction = 'continuation'
  else {
    const sorted = [...ctx.entries].sort((a, b) => a.chapter - b.chapter)
    for (const e of sorted) {
      const ph = ctx.chapterPhases[e.chapter] ?? 'empty'
      const next = inferChapterNextAction(ph)
      if (next) {
        primaryAction = next
        primaryChapter = e.chapter
        break
      }
    }
    if (!primaryAction) {
      primaryAction = 'continue'
      primaryChapter = nextChapterNumber(committed, ctx.entries)
    }
  }

  return {
    phase,
    primaryAction,
    primaryChapter,
    gates,
    blockers,
    progress: { committed, totalWithContract, percent },
    frozenBatch: ctx.state.frozenBatch,
  }
}

export function canRunAction(action: NovelStageAction, ctx: NovelBookContext, chapter?: number): NovelActionDecision {
  const blockers: string[] = []
  const ch = chapter ?? ctx.entries.find((e) => ctx.chapterPhases[e.chapter] !== 'committed')?.chapter

  if (action === 'write' || action === 'review' || action === 'commit' || action === 'polish') {
    if (ctx.castFileCount === 0) blockers.push('blocker.noCast')
  }

  if (action === 'write') {
    if (!chapter && !ch) blockers.push('blocker.noChapter')
    const target = chapter ?? ch ?? 0
    const phase = ctx.chapterPhases[target]
    if (phase !== 'contract_ready' && phase !== 'contract_draft') {
      if (phase === 'empty') blockers.push('blocker.needContract')
      else if (phase === 'drafted' || phase === 'review_fail' || phase === 'review_pass') {
        blockers.push('blocker.alreadyDrafted')
      } else if (phase === 'committed') blockers.push('blocker.alreadyCommitted')
    }
    if (
      ctx.hasVolumeOutline &&
      !ctx.batchFreezeFrozen &&
      ctx.entries.filter((e) => !e.prose && e.chapter >= target).length > 1
    ) {
      blockers.push('blocker.needBatchFreeze')
    }
  }

  if (action === 'review') {
    const target = chapter ?? ch ?? 0
    const phase = ctx.chapterPhases[target]
    if (phase !== 'drafted' && phase !== 'review_fail') {
      blockers.push('blocker.needDraft')
    }
  }

  if (action === 'commit' || action === 'polish') {
    const target = chapter ?? ch ?? 0
    const phase = ctx.chapterPhases[target]
    if (phase !== 'review_pass') {
      blockers.push('blocker.needReviewPass')
    }
  }

  if (action === 'batch-freeze') {
    if (!ctx.hasVolumeOutline) blockers.push('blocker.needVolumeOutline')
    if (ctx.batchFreezeFrozen) blockers.push('blocker.batchAlreadyFrozen')
  }

  if (action === 'batch-review') {
    const drafted = ctx.entries.filter((e) => ctx.chapterPhases[e.chapter] === 'drafted' || ctx.chapterPhases[e.chapter] === 'review_fail')
    if (!drafted.length) blockers.push('blocker.noDraftToReview')
  }

  return { allowed: blockers.length === 0, blockers }
}

export function buildConstraintFooter(
  pipeline: NovelBookPipeline,
  action: NovelStageAction,
  blockers: string[],
): string {
  const lines = [
    '---',
    '【工作台约束 — 必须遵守】',
    `- 当前书阶段：${pipeline.phase}；请求动作：${action}`,
    `- knowledge_gate：${pipeline.gates.knowledge} | asset_gate：${pipeline.gates.asset} | qc_gate：${pipeline.gates.qc}`,
  ]
  if (blockers.length) lines.push(`- 阻断项：${blockers.join('；')}`)
  lines.push('- 若门禁未 PASS，禁止写正文/Commit；用 ask_user 说明阻断项。')
  lines.push('- 完成后更新 novel-state.yaml 的 gates/blockers 字段。')
  lines.push('- read_skill novel-writing/references/preflight.md')
  lines.push('- Team：delegate_agent.goal 必须包含本消息任务正文原文，禁止改写。')
  return lines.join('\n')
}

export function buildConstrainedPrefill(
  action: NovelStageAction,
  ctx: NovelStagePrefillCtx,
  pipeline?: NovelBookPipeline,
  blockers?: string[],
): string {
  const body = buildNovelStagePrefill(action, ctx)
  if (!pipeline) return body
  return `${body}\n\n${buildConstraintFooter(pipeline, action, blockers ?? [])}`
}

export const NOVEL_PIPELINE_STEPS: { id: NovelPipelinePhase; action?: NovelStageAction }[] = [
  { id: 'init', action: 'init' },
  { id: 'setup', action: 'assets' },
  { id: 'outline', action: 'outline' },
  { id: 'batch_freeze', action: 'batch-freeze' },
  { id: 'chapter_loop', action: 'write' },
  { id: 'continuation', action: 'continuation' },
]

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

export function novelVolumesDir(bookId: string): string {
  return `novel/${bookId}/outline/volumes`
}

/** Volume number from names like v01.md, v02-三眼时间回廊.md. */
export function volumeNumFromName(name: string): number | null {
  const m = name.match(/^v(\d+)/i)
  if (!m) return null
  const n = Number.parseInt(m[1], 10)
  return Number.isFinite(n) ? n : null
}

/** Next volume to outline = max existing volume file + 1 (fallback 1). */
export function nextVolumeNumber(volumeFiles: NovelFileNode[]): number {
  let max = 0
  for (const f of volumeFiles) {
    if (f.isDir) continue
    const n = volumeNumFromName(f.name)
    if (n != null && n > max) max = n
  }
  return max + 1
}

export function novelReviewsDir(bookId: string): string {
  return `novel/${bookId}/reviews`
}

export function novelCanonDir(bookId: string): string {
  return `novel/${bookId}/canon`
}

export function novelCastDir(bookId: string): string {
  return `novel/${bookId}/canon/cast`
}

export function novelChapterSummariesPath(bookId: string): string {
  return `novel/${bookId}/continuity/chapter_summaries.md`
}

export function novelBatchFreezePath(bookId: string): string {
  return `novel/${bookId}/continuity/batch-freeze.yaml`
}

export function novelChapterReviewPath(bookId: string, chapter: number): string {
  const pad = String(chapter).padStart(3, '0')
  return `novel/${bookId}/reviews/ch${pad}-review.md`
}

export function novelChapterFilePath(bookId: string, chapter: number): string {
  const pad = String(chapter).padStart(3, '0')
  return `novel/${bookId}/chapters/ch${pad}.md`
}

export function novelChapterContractPath(bookId: string, chapter: number): string {
  const pad = String(chapter).padStart(3, '0')
  return `novel/${bookId}/chapters/ch${pad}-contract.yaml`
}

export function isNovelChapterPath(path: string): boolean {
  const p = path.replace(/\\/g, '/')
  return /\/chapters\/[^/]+\.md$/i.test(p) && !/contract\.md$/i.test(p)
}

export function isNovelContractName(name: string): boolean {
  return /contract\.(ya?ml)$/i.test(name)
}

export function isNovelContractPath(path: string): boolean {
  return /contract\.(ya?ml)$/i.test(path.replace(/\\/g, '/'))
}

/** One numbered chapter slot: optional prose (.md) + optional contract (.yaml). */
export interface NovelChapterEntry {
  chapter: number
  label: string
  prose: NovelFileNode | null
  contract: NovelFileNode | null
}

/** Sort chapter filenames like ch001.md, ch10.md naturally. */
export function sortChapterNodes(nodes: NovelFileNode[]): NovelFileNode[] {
  const files = nodes.filter((n) => !n.isDir && /\.md$/i.test(n.name) && !isNovelContractName(n.name))
  return files.slice().sort((a, b) => {
    const na = chapterNumFromName(a.name)
    const nb = chapterNumFromName(b.name)
    if (na != null && nb != null && na !== nb) return na - nb
    return a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: 'base' })
  })
}

/** Merge chapters/*.md prose and *contract.yaml under chapters/ (legacy outline/ contracts still merged if present). */
export function buildChapterEntries(...nodeLists: NovelFileNode[][]): NovelChapterEntry[] {
  const byCh = new Map<number, NovelChapterEntry>()
  const ensure = (n: number): NovelChapterEntry => {
    let e = byCh.get(n)
    if (!e) {
      e = {
        chapter: n,
        label: `ch${String(n).padStart(3, '0')}`,
        prose: null,
        contract: null,
      }
      byCh.set(n, e)
    }
    return e
  }
  for (const nodes of nodeLists) {
    for (const node of nodes) {
      if (node.isDir) continue
      const n = chapterNumFromName(node.name)
      if (n == null) continue
      if (isNovelContractName(node.name)) {
        const e = ensure(n)
        if (!e.contract) e.contract = node
      } else if (/\.md$/i.test(node.name)) {
        // Only treat chapters-dir prose as body; outline/*.md is not chapter prose.
        const p = (node.path || '').replace(/\\/g, '/')
        if (p.includes('/outline/')) continue
        ensure(n).prose = node
      }
    }
  }
  return [...byCh.values()].sort((a, b) => a.chapter - b.chapter)
}

export function sortMdNodes(nodes: NovelFileNode[]): NovelFileNode[] {
  return nodes
    .filter((n) => !n.isDir && /\.md$/i.test(n.name))
    .slice()
    .sort((a, b) => a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: 'base' }))
}

/** Outline / misc: markdown plus contract yaml so contracts under outline/ also surface. */
export function sortWorkbenchDocNodes(nodes: NovelFileNode[]): NovelFileNode[] {
  return nodes
    .filter((n) => !n.isDir && (/\.md$/i.test(n.name) || isNovelContractName(n.name)))
    .slice()
    .sort((a, b) => a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: 'base' }))
}

export function chapterNumFromName(name: string): number | null {
  const m = name.match(/(\d+)/)
  if (!m) return null
  const n = Number.parseInt(m[1], 10)
  return Number.isFinite(n) ? n : null
}

export function nextChapterNumber(
  lastCommitted: number,
  chapterFiles: NovelFileNode[] | NovelChapterEntry[],
): number {
  let maxFile = 0
  for (const f of chapterFiles) {
    if ('chapter' in f && typeof (f as NovelChapterEntry).chapter === 'number') {
      const n = (f as NovelChapterEntry).chapter
      if (n > maxFile) maxFile = n
      continue
    }
    const n = chapterNumFromName((f as NovelFileNode).name)
    if (n != null && n > maxFile) maxFile = n
  }
  return Math.max(lastCommitted, maxFile) + 1
}

/** Book-page primary CTA from files on disk (not max chapter index alone). */
export type NovelBookNextStep = {
  action: 'write' | 'contract' | 'continue'
  chapter: number
}

export function inferNovelBookNextStep(
  lastCommitted: number,
  entries: NovelChapterEntry[],
): NovelBookNextStep {
  const sorted = [...entries].sort((a, b) => a.chapter - b.chapter)
  for (const e of sorted) {
    if (e.contract && !e.prose) {
      return { action: 'write', chapter: e.chapter }
    }
  }
  if (sorted.length) {
    const max = Math.max(lastCommitted, ...sorted.map((e) => e.chapter))
    for (let n = Math.max(1, lastCommitted + 1); n <= max; n++) {
      const e = sorted.find((x) => x.chapter === n)
      if (!e?.contract) return { action: 'contract', chapter: n }
      if (!e.prose) return { action: 'write', chapter: n }
    }
  }
  const next = nextChapterNumber(lastCommitted, entries)
  if (sorted.some((e) => e.prose)) {
    return { action: 'continue', chapter: next }
  }
  return { action: 'contract', chapter: next }
}

export function buildNovelStagePrefill(action: NovelStageAction, ctx: NovelStagePrefillCtx): string {
  const bookId = (ctx.bookId ?? '').trim() || '<book-id>'
  const ch = ctx.chapter && ctx.chapter > 0 ? ctx.chapter : 0
  const chPad = ch > 0 ? String(ch).padStart(3, '0') : 'NNN'
  const chPath = ctx.chapterPath || `novel/${bookId}/chapters/ch${chPad}.md`
  const root = `novel/${bookId}`
  const vol = ctx.volume && ctx.volume > 0 ? ctx.volume : 0
  const volPad = vol > 0 ? String(vol).padStart(2, '0') : 'NN'

  switch (action) {
    case 'init':
      return [
        '开一本新书并立项：',
        '- 先用 ask_user 澄清题材、读者承诺、篇幅/平台、禁忌（一次一问即可）。',
        '- 创建 novel/<book-id>/ 标准英文树：novel-state.yaml、book-bible.md、canon/(+cast/)、outline/(+volumes/)、chapters/、continuity/、reviews/（可选 extras/、_archive/）。',
        '- 角色先 status=candidate，经我确认后再变 canon；确认前不要写正文。',
        '- 落盘后更新 novel-state（stage=init 或 outline）。',
        '- 按 read_skill novel-writing/references/init.md 与 project-layout.md 执行。',
      ].join('\n')
    case 'outline':
      return [
        `书目录：${root}/`,
        '基于现有 book-bible / canon 产出总纲与卷纲（不写章节正文）：',
        `- 总纲 ${root}/outline/book_outline.md（模板 book-outline.md：一句话故事 / 读者承诺 / 分卷结构表 / 主线伏笔 / 结局方向）。`,
        `- 卷纲 ${root}/outline/volumes/vNN.md（模板 volume-outline.md：卷目标 / 核心冲突 / 节奏锚点 / 章纲表）。`,
        '章纲表固定四列「章号 | 一句任务 | 爽点 | 钩子」，连续 3 行无爽点必须重排（KB 节奏与结构）。',
        '每卷卷纲写完停下来等我确认。',
        '按 read_skill novel-writing/references/outline.md 执行。',
      ].join('\n')
    case 'volume':
      return [
        `为本书写第 ${vol || 'N'} 卷卷纲（书目录：${root}/）。`,
        `唯一落盘：${root}/outline/volumes/v${volPad}.md（模板 volume-outline.md，先 read_skill novel-writing/assets/templates/volume-outline.md）。`,
        '先读 outline/book_outline.md 与 canon/ 相关设定、continuity/ 未回收伏笔，保持与前后卷衔接。',
        '内容：卷目标一句话 / 核心冲突（对手+代价）/ 节奏锚点（卷高潮章位、中点反转、伏笔埋收）/ 章纲表（固定四列：章号 | 一句任务 | 爽点 | 钩子）。',
        '章纲表连续 3 行无爽点必须重排；卷纲只到章纲表为止，不写章合同、不写正文。',
        '写完停下来等我确认，再进入章合同阶段。',
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
        `唯一落盘：${root}/chapters/ch${chPad}-contract.yaml（YAML；模板 chapter-contract.yaml）。`,
        `可选 table_upsert chapter_contracts 作索引（book_id=${bookId}，file 指向该 yaml），不能代替文件。`,
        '含：purpose、beats / forbidden、pleasure_point、state_deltas、伏笔、hook(type+out)、word_target、连续性风险；status=proposed。',
        '里程碑章或本批首章需 ask_user 接受后再进入写作。',
        '按 read_skill novel-writing/references/chapter-contract.md 执行。',
      ].join('\n')
    case 'write':
      return [
        `写第 ${ch || 'N'} 章正文到 ${chPath}。`,
        '前提：该章合同已 accepted（否则先补合同）。',
        '流程：preflight → knowledge/asset 门 → scene-routing（若 beats 有场景标签）→ 按合同草稿 → 不要跳过审稿与 Commit。',
        'P0 审稿不过不得定稿；不要只在对话里贴正文。',
        '按 read_skill novel-writing/references/preflight.md 与 chapter-write.md 执行。',
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
        `审阅 ${chPath}：按六透镜 + ReaderPull + StrongConstraints + 去 AI 味 P0/P1 出审查报告，写入 ${root}/reviews/，`,
        'blocking 记入 continuity_issues；SCORES 段按 qc_profile 加权。',
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
        '更新：章节定稿文件、table_*（timeline / foreshadows / characters / contracts）、memory、novel-state。',
        '追加 continuity/chapter_summaries.md 固定 5 字段块（事件 / 状态变化 / 伏笔 / 钩子 / 下章指向）。',
        '关闭已修复的 continuity_issues。每满 10 个已提交章写 continuity/phase-NN.md。',
        '按 read_skill novel-writing/references/continuity-commit.md 执行。完成=工具证据，勿口头宣称。',
      ].join('\n')
    case 'batch-freeze':
      const bFrom = ctx.batchFrom && ctx.batchFrom > 0 ? ctx.batchFrom : 1
      const bTo = ctx.batchTo && ctx.batchTo > 0 ? ctx.batchTo : 8
      return [
        `批次细纲冻结（书：${root}/，第 ${bFrom}–${bTo} 章）。`,
        `落盘 continuity/batch-freeze.yaml（模板 batch-freeze.yaml）；硬逻辑审核后 status=frozen。`,
        '冻结前禁止批量写正文。用户确认后更新 novel-state frozen_batch 与 artifacts.batch_freeze。',
        '按 read_skill novel-writing/references/batch-freeze.md 执行。',
      ].join('\n')
    case 'continuation':
      return [
        `续写/接手本书（${root}/）：CP1 反向解析 → style-fingerprint.md → CP2 卡点诊断 → CP3 Frozen_Canon。`,
        'Frozen_Canon 未确认禁止写正文；首次续写 500–1000 字试写待确认。',
        '按 read_skill novel-writing/references/continuation.md 执行。',
      ].join('\n')
    case 'batch-review':
      return [
        `批量审稿（书：${root}/）：最近 1–5 章有正文但未 PASS 的章节。`,
        '每章独立 review 文件；更新 gates.qc 与 blockers。',
        '按 read_skill novel-writing/references/review-gates.md 与 preflight.md 执行。',
      ].join('\n')
    case 'preflight':
      return [
        `写前预检（书：${root}/）：读 state、合同、摘要、cast、foreshadows；写 continuity/preflight-log.md 一行回执。`,
        '更新 novel-state gates/blockers。阻断则停止并 ask_user。',
        '按 read_skill novel-writing/references/preflight.md 执行。',
      ].join('\n')
    default:
      return ''
  }
}
