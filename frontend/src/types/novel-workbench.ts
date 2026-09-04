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

/** Coarse phase for constraint footer / legacy. */
export type NovelPipelinePhase = 'init' | 'setup' | 'outline' | 'writing' | 'review' | 'idle'

/** Eight visible pipeline steps (workbench stepper). */
export type NovelPipelineStepId =
  | 'init'
  | 'setup'
  | 'outline'
  | 'volume'
  | 'contract'
  | 'write'
  | 'review'
  | 'commit'

export type NovelChapterPhase =
  | 'empty'
  | 'contract_draft'
  | 'contract_ready'
  | 'drafted'
  | 'review_fail'
  | 'review_pass'
  | 'committed'

/** Workbench actions; each maps to a focused novel-* skill. */
export type NovelStageAction =
  | 'init'
  | 'outline'
  | 'volume'
  | 'assets'
  | 'goldfinger'
  | 'contract'
  | 'write'
  | 'continue'
  | 'dialogue'
  | 'hook'
  | 'reversal'
  | 'review'
  | 'polish'
  | 'commit'
  | 'review-polish-commit'
  | 'batch-freeze'
  | 'continuation'
  | 'batch-review'
  | 'preflight'

export type NovelSkillId = 'novel-setup' | 'novel-plan' | 'novel-write' | 'novel-review'

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
  /** Active step among the 8-step rail. */
  step: NovelPipelineStepId
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

export function parseContractYaml(raw: string): { status: string; title: string; unitId: string } {
  const status = yamlScalar(raw, 'status').toLowerCase()
  return {
    status: status || 'proposed',
    title: yamlScalar(raw, 'title_working'),
    unitId: yamlScalar(raw, 'unit_id'),
  }
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
    // PASS stub optional: contract reviewed (post-review, pre-commit) or legacy PASS file
    if (verdict === 'PASS' || contractStatus === 'reviewed') return 'review_pass'
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
      return 'review-polish-commit'
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
  if (ctx.state.stage === 'idle') return 'idle'
  const art = ctx.state
  // Prefer artifacts / disk readiness over coarse stage string
  if (!ctx.hasBookOutline && art.stage === 'init') return 'init'
  if (!ctx.hasBookOutline && ctx.castFileCount === 0) return 'init'
  if (!ctx.hasBookOutline) return ctx.castFileCount === 0 ? 'setup' : 'outline'
  const phases = Object.values(ctx.chapterPhases)
  if (phases.some((p) => p === 'drafted' || p === 'review_fail' || p === 'review_pass')) {
    return 'review'
  }
  if (ctx.entries.length > 0 || ctx.hasVolumeOutline) return 'writing'
  if (ctx.castFileCount === 0) return 'setup'
  return 'outline'
}

/** Map book context to one of 8 visible pipeline steps. */
export function inferPipelineStep(ctx: NovelBookContext): NovelPipelineStepId {
  if (ctx.state.stage === 'idle') return 'commit'
  if (!ctx.hasBookOutline && ctx.castFileCount === 0 && ctx.state.stage === 'init') return 'init'
  if (!ctx.hasBookOutline && ctx.castFileCount === 0) return 'setup'
  if (!ctx.hasBookOutline) return 'outline'
  if (!ctx.hasVolumeOutline) return 'volume'

  const sorted = [...ctx.entries].sort((a, b) => a.chapter - b.chapter)
  for (const e of sorted) {
    const ph = ctx.chapterPhases[e.chapter] ?? 'empty'
    if (ph === 'empty' || ph === 'contract_draft') return 'contract'
    if (ph === 'contract_ready') return 'write'
    if (ph === 'drafted' || ph === 'review_fail') return 'review'
    if (ph === 'review_pass') return 'commit'
  }
  // Has volume but no chapter work yet → next is contract
  if (ctx.entries.length === 0) return 'contract'
  return 'write'
}

export function computeBookPipeline(ctx: NovelBookContext): NovelBookPipeline {
  const phase = inferBookPipelinePhase(ctx)
  const step = inferPipelineStep(ctx)
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

  switch (step) {
    case 'init':
      primaryAction = 'init'
      break
    case 'setup':
      primaryAction = 'assets'
      break
    case 'outline':
      primaryAction = 'outline'
      break
    case 'volume':
      primaryAction = 'volume'
      break
    case 'contract':
    case 'write':
    case 'review':
    case 'commit': {
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
        if (step === 'contract' || step === 'write') {
          primaryAction = 'contract'
          primaryChapter = nextChapterNumber(committed, ctx.entries)
        } else {
          primaryAction = 'continue'
          primaryChapter = nextChapterNumber(committed, ctx.entries)
        }
      }
      break
    }
  }

  return {
    phase,
    step,
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

  if (
    action === 'write' ||
    action === 'review' ||
    action === 'commit' ||
    action === 'polish' ||
    action === 'review-polish-commit'
  ) {
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

  if (action === 'review-polish-commit') {
    if (!chapter && !ch) blockers.push('blocker.noChapter')
    const target = chapter ?? ch ?? 0
    const phase = ctx.chapterPhases[target]
    if (phase === 'empty' || phase === 'contract_draft' || phase === 'contract_ready') {
      blockers.push('blocker.needDraft')
    }
    if (phase === 'committed') blockers.push('blocker.alreadyCommitted')
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

export function novelActionSkillId(action: NovelStageAction): NovelSkillId {
  switch (action) {
    case 'init':
      return 'novel-setup'
    case 'assets':
    case 'goldfinger':
    case 'outline':
    case 'volume':
      return 'novel-plan'
    case 'review':
    case 'polish':
    case 'commit':
    case 'review-polish-commit':
    case 'batch-review':
      return 'novel-review'
    default:
      return 'novel-write'
  }
}

/**
 * Composer load line: skill + intent only.
 * Which reference / template / KB theme to open lives in each skill's
 * Intent→Load table — do not hardcode skillRefs here (skill updates would desync).
 */
export function formatLoadProtocol(action: NovelStageAction): string {
  const skillId = novelActionSkillId(action)
  return `技能 ${skillId} · 意图 ${action} — 按该技能 Intent→Load 表 read_skill；写正文先跑 gate preflight，只消费 ### CONTEXT。`
}

export function buildConstraintFooter(
  pipeline: NovelBookPipeline,
  action: NovelStageAction,
  blockers: string[],
): string {
  const skillId = novelActionSkillId(action)
  const lines = [
    '---',
    `阶段 ${pipeline.phase}/${pipeline.step} · 动作 ${action} · 技能 ${skillId}`,
    `gates knowledge=${pipeline.gates.knowledge} asset=${pipeline.gates.asset} qc=${pipeline.gates.qc}`,
  ]
  if (blockers.length) lines.push(`阻断：${blockers.join('；')}`)
  lines.push('Team：delegate_agent.goal 须含本消息任务原文。')
  return lines.join('\n')
}

export function buildConstrainedPrefill(
  action: NovelStageAction,
  ctx: NovelStagePrefillCtx,
  pipeline?: NovelBookPipeline,
  blockers?: string[],
): string {
  const body = buildNovelStagePrefill(action, ctx)
  const skillLine = formatLoadProtocol(action)
  const parts = [`【任务】\n${body}`, skillLine]
  if (pipeline) {
    parts.push(buildConstraintFooter(pipeline, action, blockers ?? []))
  }
  return parts.join('\n\n')
}

/** Stepper is progress-only; clicks do not inject Composer. */
export const NOVEL_PIPELINE_STEPS: { id: NovelPipelineStepId; action?: NovelStageAction }[] = [
  { id: 'init' },
  { id: 'setup' },
  { id: 'outline' },
  { id: 'volume' },
  { id: 'contract' },
  { id: 'write' },
  { id: 'review' },
  { id: 'commit' },
]

export function novelActiveBookPath(): string {
  return 'novel/.active-book'
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

export function novelVolumesDir(bookId: string): string {
  return `novel/${bookId}/outline/volumes`
}

/** Volume number from names like v01.md, v02-三眼时间回廊.md, volume01-chapter-index.md. */
export function volumeNumFromName(name: string): number | null {
  const m = name.match(/^(?:volume|vol|v)0*(\d+)/i)
  if (!m) return null
  const n = Number.parseInt(m[1], 10)
  return Number.isFinite(n) ? n : null
}

export function isBookOutlineName(name: string): boolean {
  return name.toLowerCase() === 'book_outline.md'
}

/** Volume-level outline at outline root or outline/volumes/. */
export function isVolumeOutlineName(name: string): boolean {
  if (isBookOutlineName(name)) return false
  if (volumeNumFromName(name) != null) return true
  return /chapter-index/i.test(name)
}

export function mergeVolumeOutlineFiles(
  outlineRoot: NovelFileNode[],
  volumeDir: NovelFileNode[],
): NovelFileNode[] {
  const fromDir = volumeDir.filter((n) => !n.isDir)
  const seen = new Set(fromDir.map((n) => n.name.toLowerCase()))
  const extras = outlineRoot.filter(
    (n) => !n.isDir && isVolumeOutlineName(n.name) && !seen.has(n.name.toLowerCase()),
  )
  return [...fromDir, ...extras]
}

export interface BookOutlineVolumeRow {
  vol: string
  goal: string
  climax: string
  twist: string
}

export interface VolumeUnitRow {
  id: string
  range: string
  purpose: string
}

function markdownTableCells(line: string): string[] {
  const trimmed = line.trim()
  if (!trimmed.startsWith('|')) return []
  return trimmed
    .split('|')
    .slice(1, -1)
    .map((c) => c.trim())
}

function isMarkdownSepRow(cells: string[]): boolean {
  return cells.length > 0 && cells.every((c) => /^:?-+:?$/.test(c) || c === '')
}

/** Parse 分卷结构 rows from book_outline.md. */
export function parseBookOutlineVolumeRows(md: string): BookOutlineVolumeRow[] {
  const rows: BookOutlineVolumeRow[] = []
  let inTable = false
  for (const line of md.split(/\r?\n/)) {
    const cells = markdownTableCells(line)
    if (!cells.length) {
      if (inTable && rows.length) break
      continue
    }
    if (isMarkdownSepRow(cells)) continue
    if (cells[0] === '卷' || cells.includes('卷目标')) {
      inTable = true
      continue
    }
    if (!inTable) continue
    if (!cells[0]) continue
    rows.push({
      vol: cells[0],
      goal: cells[1] ?? '',
      climax: cells[2] ?? '',
      twist: cells[3] ?? '',
    })
  }
  return rows
}

/** Parse 剧情单元 cards, or a legacy unit/chapter-index table, from a volume outline. */
export function parseChapterRange(s: string): { from: number; to: number } | null {
  const m = s.match(/ch?\s*(\d+)\s*[-–~至到]\s*ch?\s*(\d+)/i) || s.match(/(\d+)\s*[-–~至到]\s*(\d+)/)
  if (!m) return null
  const from = Number.parseInt(m[1], 10)
  const to = Number.parseInt(m[2], 10)
  if (!Number.isFinite(from) || !Number.isFinite(to)) return null
  return { from: Math.min(from, to), to: Math.max(from, to) }
}

function stripMdInline(s: string): string {
  return s.replace(/^`+|`+$/g, '').trim()
}

/** Prefer unit cards (`### 剧情单元` / `- 单元ID：`); fall back to markdown tables. */
export function parseVolumeUnitRows(md: string): VolumeUnitRow[] {
  const cards = parseVolumeUnitCards(md)
  if (cards.length) return cards
  return parseVolumeUnitTable(md)
}

function parseVolumeUnitCards(md: string): VolumeUnitRow[] {
  const rows: VolumeUnitRow[] = []
  let cur: VolumeUnitRow | null = null
  const flush = () => {
    if (cur && (cur.id || cur.range || cur.purpose)) {
      if (!cur.id && cur.range) cur.id = cur.range
      rows.push(cur)
    }
    cur = null
  }

  for (const raw of md.split(/\r?\n/)) {
    const line = raw.trim()
    const heading = line.match(/^#{2,4}\s*剧情单元\s*(.+)?$/i)
    if (heading) {
      flush()
      const rest = (heading[1] || '').trim()
      cur = { id: rest || '', range: '', purpose: '' }
      continue
    }
    const idLine = line.match(/^[-*]\s*单元ID[：:]\s*(.+)$/i)
    if (idLine) {
      if (!cur) cur = { id: '', range: '', purpose: '' }
      cur.id = stripMdInline(idLine[1])
      continue
    }
    if (!cur) continue
    const rangeLine = line.match(/^[-*]\s*章范围[：:]\s*(.+)$/i)
    if (rangeLine) {
      cur.range = stripMdInline(rangeLine[1])
      continue
    }
    const purposeLine = line.match(/^[-*]\s*单元功能[（(][^）)]*[）)]?[：:]\s*(.+)$/i)
      || line.match(/^[-*]\s*单元功能[：:]\s*(.+)$/i)
    if (purposeLine) {
      cur.purpose = stripMdInline(purposeLine[1])
      continue
    }
    // Next major section ends the unit block list.
    if (/^##\s+/.test(line) && !/^##\s*剧情单元/i.test(line)) {
      flush()
      break
    }
  }
  flush()
  return rows.filter((r) => Boolean(r.id || r.range))
}

function parseVolumeUnitTable(md: string): VolumeUnitRow[] {
  const rows: VolumeUnitRow[] = []
  let mode: 'unit' | 'legacy' | null = null
  for (const line of md.split(/\r?\n/)) {
    const cells = markdownTableCells(line)
    if (!cells.length) {
      if (mode && rows.length) break
      continue
    }
    if (isMarkdownSepRow(cells)) continue
    if (cells[0] === '单元' || cells.includes('章范围')) {
      mode = 'unit'
      continue
    }
    if (cells[0] === '章' || cells.includes('一句任务')) {
      mode = 'legacy'
      continue
    }
    if (!mode) continue
    if (!cells[0]) continue
    if (mode === 'unit') {
      rows.push({
        id: cells[0],
        range: cells[1] ?? '',
        purpose: cells[2] ?? '',
      })
    } else {
      rows.push({
        id: cells[0],
        range: cells[0],
        purpose: cells[1] ?? '',
      })
    }
  }
  return rows
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

/** Display name for a 设定 file. Known canon filenames get short labels; cast cards drop the numeric prefix. */
export function setupDocLabel(name: string): string {
  const key = name.toLowerCase()
  const known: Record<string, string> = {
    'book-bible.md': 'bible',
    'world.md': 'world',
    'glossary.md': 'glossary',
    'reveal-schedule.md': 'reveal',
    'writing-rules.md': 'rules',
    'platform-positioning.md': 'platform',
    'goldfinger.md': 'goldfinger',
  }
  if (known[key]) return known[key]
  return name.replace(/^\d+-/, '').replace(/\.md$/i, '')
}

export function novelCastDir(bookId: string): string {
  return `novel/${bookId}/canon/cast`
}

export function novelChapterSummariesPath(bookId: string): string {
  // Legacy alias — prefer ledger
  return `novel/${bookId}/continuity/ledger.md`
}

export function novelLedgerPath(bookId: string): string {
  return `novel/${bookId}/continuity/ledger.md`
}

export function novelBatchFreezePath(bookId: string): string {
  // Legacy path kept for soft-read; freeze now lives in novel-state.yaml
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
  return `novel/${bookId}/chapters/ch${pad}-outline.yaml`
}

export function isNovelChapterPath(path: string): boolean {
  const p = path.replace(/\\/g, '/')
  return /\/chapters\/[^/]+\.md$/i.test(p) && !/(?:contract|outline)\.md$/i.test(p)
}

/** Chapter outline YAML under chapters/ (canonical *-outline.yaml; legacy *-contract.yaml). */
export function isNovelContractName(name: string): boolean {
  return /^ch\d+-(?:outline|contract)\.(ya?ml)$/i.test(name)
}

export function isNovelContractPath(path: string): boolean {
  const p = path.replace(/\\/g, '/')
  return /\/chapters\/ch\d+-(?:outline|contract)\.(ya?ml)$/i.test(p)
}

/** One numbered chapter slot: optional prose (.md) + optional chapter outline (.yaml). */
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

/** Merge chapters/*.md prose and chapter-outline YAML under chapters/ (canonical *-outline.yaml; legacy *-contract.yaml). */
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
        const isCanonical = /outline\.(ya?ml)$/i.test(node.name)
        const existingIsCanonical = e.contract
          ? /outline\.(ya?ml)$/i.test(e.contract.name)
          : false
        if (!e.contract || (isCanonical && !existingIsCanonical)) {
          e.contract = node
        }
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

/** Outline / misc: markdown plus chapter-outline yaml so legacy outlines under outline/ also surface. */
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
  // Keep intent + project paths only. Which skill reference / template / KB
  // theme to open lives in the skill Intent→Load table (see formatLoadProtocol).

  switch (action) {
    case 'init':
      return [
        '开一本新书并立项。',
        '用一次 ask_user 收齐：题材、读者承诺、篇幅/平台、POV、禁忌。',
        '落盘标准树到 novel/<book-id>/。新角色先 candidate；卷纲批准时一并 promote。',
      ].join('\n')
    case 'outline':
      return [
        `书目录：${root}/`,
        `产出总纲 ${root}/outline/book_outline.md（不写章节正文）。`,
        '卷纲另做；单章任务只进章纲。',
      ].join('\n')
    case 'volume':
      return [
        `为第 ${vol || 'N'} 卷写卷纲 → ${root}/outline/volumes/v${volPad}.md`,
        '止于剧情单元卡；不写章纲/正文。写完等我确认（本卷点名人物一并 canon）。',
      ].join('\n')
    case 'assets':
      return [
        `书目录：${root}/`,
        '整理/补全人物卡与世界观到 canon/。新实体先 candidate；卷纲批准时 promote。',
      ].join('\n')
    case 'goldfinger':
      return [
        `书目录：${root}/`,
        '设计或修订金手指，默认写入主角 cast 卡。',
      ].join('\n')
    case 'contract':
      return [
        `为第 ${ch || 'N'} 章写章纲 → ${root}/chapters/ch${chPad}-outline.yaml`,
        '从本卷纲单元卡下推；必填 unit_id。就绪后 status=accepted。',
      ].join('\n')
    case 'write':
      return [
        `写第 ${ch || 'N'} 章正文到 ${chPath}。`,
        '先 gate preflight，只消费 ### CONTEXT + 本章纲；落盘正文。',
      ].join('\n')
    case 'continue':
      return [
        `接着写下一章（书：${root}/）。`,
        '补章纲 → gate CONTEXT → 正文；定稿用审→润→Commit。',
      ].join('\n')
    case 'dialogue':
      return [
        `加强第 ${ch || 'N'} 章对话（${chPath}）。`,
        '按章纲 beats 写出可辨声口；落盘到该章正文。',
      ].join('\n')
    case 'hook':
      return [
        `为第 ${ch || 'N'} 章改写章末悬念钩（${chPath}）。`,
        '章末 hook 须可执行；写入章纲并落到正文。',
      ].join('\n')
    case 'reversal':
      return [
        `为第 ${ch || 'N'} 章加一处反转（${chPath}）。`,
        '须服务章纲 purpose；先改章纲再改正文。',
      ].join('\n')
    case 'review':
      return [
        `审阅 ${chPath}。`,
        '先 gate precommit。PASS：只更新 gates.qc，不写 review 文件。FAIL/深审：写 reviews/ 全文。',
      ].join('\n')
    case 'polish':
      return [
        `对 ${chPath} 做去 AI 味润色并落盘。`,
        'gate scan-deslop 定位后按行号改；不改情节 Canon。',
      ].join('\n')
    case 'commit':
      return [
        ch > 0
          ? `对第 ${ch} 章（${chPath}）做 Continuity Commit。`
          : `对当前进度做 Continuity Commit（书：${root}/）。`,
        '一次补丁更新 ledger + 章纲 reviewed + novel-state；再 gate postcommit。',
      ].join('\n')
    case 'review-polish-commit':
      return [
        ch > 0
          ? `对第 ${ch} 章（${chPath}）串行：审 → 可选润色 → Commit。`
          : `对当前章节串行：审 → 可选润色 → Commit（书：${root}/）。`,
        'PASS 不写 review 文件；FAIL 才落盘。Commit = ledger + 章纲 + state + postcommit。',
      ].join('\n')
    case 'batch-freeze': {
      const bFrom = ctx.batchFrom && ctx.batchFrom > 0 ? ctx.batchFrom : 1
      const bTo = ctx.batchTo && ctx.batchTo > 0 ? ctx.batchTo : 8
      return [
        `批次冻结（书：${root}/，第 ${bFrom}–${bTo} 章）。`,
        '只更新 novel-state.yaml 的 frozen_batch；按单元章范围自动冻结。',
      ].join('\n')
    }
    case 'continuation':
      return [
        `续写/接手本书（${root}/）。`,
        'Frozen_Canon 未确认禁止写正文。',
      ].join('\n')
    case 'batch-review':
      return [
        `批量审稿（书：${root}/）：最近有正文未定稿的章。`,
        'PASS 不落盘；FAIL 写 reviews/。',
      ].join('\n')
    case 'preflight':
      return [
        `写前预检（书：${root}/，章 ${ch || 'N'}）。`,
        'exec_shell gate preflight；回报 ### CONTEXT 与 VERDICT。',
      ].join('\n')
    default:
      return ''
  }
}
