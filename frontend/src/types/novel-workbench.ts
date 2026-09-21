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
  activeUnit: string
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
  | 'expand'
  | 'review'
  | 'polish'
  | 'commit'
  | 'review-polish-commit'
  | 'continuation'
  | 'preflight'

export type NovelSkillId = 'novel-setup' | 'novel-plan' | 'novel-write' | 'novel-review'

export interface NovelStagePrefillCtx {
  bookId?: string
  chapter?: number
  chapterPath?: string
  unitId?: string
  unitPath?: string
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
  primaryUnit?: string
  primaryChapter?: number
  gates: { knowledge: GateStatus; asset: GateStatus; qc: GateStatus }
  blockers: string[]
  progress: { committed: number; totalWithContract: number; percent: number }
  frozenBatch: { from: number; to: number } | null
}

export interface NovelBookContext {
  bookId: string
  state: NovelExtendedState
  entries: NovelUnitEntry[]
  unitPhases: Record<string, NovelChapterPhase>
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
  if (v.startsWith('"') || v.startsWith("'")) {
    const q = v[0]
    const end = v.indexOf(q, 1)
    v = end > 0 ? v.slice(1, end) : v.slice(1)
  } else {
    const hash = v.search(/\s+#/)
    if (hash >= 0) v = v.slice(0, hash).trim()
  }
  if (v === '""' || v === "''") return ''
  return v
}

function yamlNestedScalar(raw: string, parent: string, key: string): string {
  // Capture indented block under `parent:` until next top-level key.
  // Do not use `$` in a non-greedy lookahead — it stops at the first nested line EOL.
  const blockRe = new RegExp(`^${parent}:\\s*\\n((?:[ \\t].*\\n?)*)`, 'm')
  const block = raw.match(blockRe)
  if (!block) return ''
  const re = new RegExp(`^[ \\t]+${key}:\\s*(.*)$`, 'm')
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
    activeUnit: yamlScalar(raw, 'active_unit'),
    frozenBatch,
    batchFreezeArtifact,
    gates: parseGateBlock(raw),
    blockers,
  }
}

/** Structured fields from chapters/chNNN-outline.yaml (best-effort scalars). */
export interface NovelContractFields {
  status: string
  title: string
  unitId: string
  scene: string
  purpose: string
  pleasurePoint: string
  microPayoff: string
  emotionLine: string
  wordTarget: string
  hookType: string
  hookOut: string
}

export function parseContractYaml(raw: string): NovelContractFields {
  const status = yamlScalar(raw, 'status').toLowerCase()
  return {
    status: status || 'proposed',
    title: yamlScalar(raw, 'title_working'),
    unitId: yamlScalar(raw, 'unit_id'),
    scene: yamlScalar(raw, 'scene'),
    purpose: yamlScalar(raw, 'purpose'),
    pleasurePoint: yamlScalar(raw, 'pleasure_point'),
    microPayoff: yamlScalar(raw, 'micro_payoff'),
    emotionLine: yamlScalar(raw, 'emotion_line'),
    wordTarget: yamlScalar(raw, 'word_target'),
    hookType: yamlNestedScalar(raw, 'hook', 'type'),
    hookOut: yamlNestedScalar(raw, 'hook', 'out'),
  }
}

/** True when the chapter still needs author/agent work (not yet committed). */
export function isChapterPhasePending(phase: NovelChapterPhase): boolean {
  return phase !== 'committed'
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
  if (ctx.state.stage === 'idle') return 'idle'
  const art = ctx.state
  if (!ctx.hasBookOutline && art.stage === 'init') return 'init'
  if (!ctx.hasBookOutline && ctx.castFileCount === 0) return 'init'
  if (!ctx.hasBookOutline) return ctx.castFileCount === 0 ? 'setup' : 'outline'
  const phases = Object.values(ctx.unitPhases)
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

  const sorted = [...ctx.entries].sort(compareUnits)
  for (const e of sorted) {
    const ph = ctx.unitPhases[e.unitId] ?? 'empty'
    if (ph === 'empty' || ph === 'contract_draft') return 'contract'
    if (ph === 'contract_ready') return 'write'
    if (ph === 'drafted' || ph === 'review_fail') return 'review'
    if (ph === 'review_pass') return 'commit'
  }
  if (ctx.entries.length === 0) return 'contract'
  return 'write'
}

function compareUnits(a: NovelUnitEntry, b: NovelUnitEntry): number {
  if (a.volume !== b.volume) return a.volume - b.volume
  return a.index - b.index
}

export function computeBookPipeline(ctx: NovelBookContext): NovelBookPipeline {
  const phase = inferBookPipelinePhase(ctx)
  const step = inferPipelineStep(ctx)
  const committed = ctx.entries.filter((e) => ctx.unitPhases[e.unitId] === 'committed').length
  const totalWithContract = ctx.entries.filter((e) => e.outline || e.prose).length
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
  let primaryUnit: string | undefined

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
      const sorted = [...ctx.entries].sort(compareUnits)
      for (const e of sorted) {
        const ph = ctx.unitPhases[e.unitId] ?? 'empty'
        const next = inferChapterNextAction(ph)
        if (next) {
          primaryAction = next
          primaryUnit = e.unitId
          break
        }
      }
      if (!primaryAction) {
        primaryAction = step === 'review' || step === 'commit' ? 'continue' : 'contract'
      }
      break
    }
  }

  return {
    phase,
    step,
    primaryAction,
    primaryUnit,
    gates,
    blockers,
    progress: { committed, totalWithContract, percent },
    frozenBatch: ctx.state.frozenBatch,
  }
}

export function canRunAction(
  action: NovelStageAction,
  ctx: NovelBookContext,
  unitId?: string,
): NovelActionDecision {
  const blockers: string[] = []
  const pending = [...ctx.entries].sort(compareUnits).find((e) => ctx.unitPhases[e.unitId] !== 'committed')
  const uid = unitId ?? pending?.unitId ?? ''

  if (
    action === 'write' ||
    action === 'expand' ||
    action === 'review' ||
    action === 'commit' ||
    action === 'polish' ||
    action === 'review-polish-commit'
  ) {
    if (ctx.castFileCount === 0) blockers.push('blocker.noCast')
  }

  const phaseOf = (id: string) => (id ? ctx.unitPhases[id] : undefined)

  if (action === 'write') {
    if (!uid) blockers.push('blocker.noChapter')
    const phase = phaseOf(uid)
    if (phase !== 'contract_ready') {
      if (phase === 'empty' || phase === 'contract_draft' || !phase) blockers.push('blocker.needContract')
      else if (phase === 'drafted' || phase === 'review_fail' || phase === 'review_pass') {
        blockers.push('blocker.alreadyDrafted')
      } else if (phase === 'committed') blockers.push('blocker.alreadyCommitted')
    }
  }

  if (action === 'expand' || action === 'review') {
    const phase = phaseOf(uid)
    if (action === 'expand') {
      if (phase !== 'drafted' && phase !== 'review_fail' && phase !== 'review_pass') {
        blockers.push('blocker.needDraft')
      }
      if (phase === 'committed') blockers.push('blocker.alreadyCommitted')
    } else if (phase !== 'drafted' && phase !== 'review_fail') {
      blockers.push('blocker.needDraft')
    }
  }

  if (action === 'commit' || action === 'polish') {
    const phase = phaseOf(uid)
    if (phase !== 'review_pass') blockers.push('blocker.needReviewPass')
  }

  if (action === 'review-polish-commit') {
    if (!uid) blockers.push('blocker.noChapter')
    const phase = phaseOf(uid)
    if (phase === 'empty' || phase === 'contract_draft' || phase === 'contract_ready' || !phase) {
      blockers.push('blocker.needDraft')
    }
    if (phase === 'committed') blockers.push('blocker.alreadyCommitted')
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
    case 'expand':
    case 'review':
    case 'polish':
    case 'commit':
    case 'review-polish-commit':
    case 'continuation':
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
  if (skillId === 'novel-write' && (action === 'write' || action === 'continue' || action === 'preflight')) {
    return `技能 ${skillId} · 意图 ${action} — 按该技能 Intent→Load 表 read_skill；写正文先跑 gate preflight --unit，只消费 ### CONTEXT；落盘一份单元正文后停下（扩写/润色/定稿另轮）。`
  }
  if (skillId === 'novel-review') {
    return `技能 ${skillId} · 意图 ${action} — 按该技能 Intent→Load 表 read_skill；定稿车道（扩写/审/润/Commit），与写作首稿分 turn。`
  }
  return `技能 ${skillId} · 意图 ${action} — 按该技能 Intent→Load 表 read_skill。`
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

/** Chapter outline YAML under chapters/ (`chNNN-outline.yaml`). */
export function isNovelContractName(name: string): boolean {
  return /^ch\d+-outline\.(ya?ml)$/i.test(name)
}

export function isNovelContractPath(path: string): boolean {
  const p = path.replace(/\\/g, '/')
  return /\/chapters\/ch\d+-outline\.(ya?ml)$/i.test(p)
}

export interface NovelUnitEntry {
  unitId: string
  volume: number
  index: number
  label: string
  chapterFrom: number
  chapterTo: number
  outline: NovelFileNode | null
  prose: NovelFileNode | null
}

export interface NovelUnitScene {
  id: string
  beat: string
  chapter: number
}

export interface NovelUnitChapterCut {
  chapter: number
  title: string
  cutHook: string
  wordShare: string
}

export interface NovelUnitOutlineFields {
  unitId: string
  status: string
  title: string
  functionText: string
  desire: string
  obstacle: string
  pleasure: string
  wordTarget: string
  hookType: string
  hookOut: string
  scenes: NovelUnitScene[]
  chapters: NovelUnitChapterCut[]
}

export interface NovelUnitProseSection {
  chapter: number
  title: string
  body: string
}

/** Plain text for pasting one chapter (title + body, no markdown heading markers). */
export function formatChapterPlain(section: NovelUnitProseSection): string {
  const head = section.title.trim()
    ? `第${section.chapter}章 ${section.title.trim()}`
    : `第${section.chapter}章`
  const body = section.body.replace(/\r\n/g, '\n').trim()
  return body ? `${head}\n\n${body}\n` : `${head}\n`
}

/** Plain text for pasting a whole unit; chapters joined with a lone --- line. */
export function formatUnitProsePlain(sections: NovelUnitProseSection[]): string {
  return sections.map(formatChapterPlain).join('\n---\n\n')
}

export function unitMetaFromName(name: string): { unitId: string; volume: number; index: number } | null {
  const m = name.match(/^(v(\d+)-U(\d+))(?:\.(?:ya?ml|md))?$/i)
  if (!m) return null
  return { unitId: m[1], volume: Number(m[2]), index: Number(m[3]) }
}

export function isNovelUnitOutlineName(name: string): boolean {
  return /^v\d+-U\d+\.ya?ml$/i.test(name)
}

export function isNovelUnitProseName(name: string): boolean {
  return /^v\d+-U\d+\.md$/i.test(name)
}

export function novelUnitOutlinePath(bookId: string, unitId: string): string {
  return `novel/${bookId}/outline/units/${unitId}.yaml`
}

export function novelUnitProsePath(bookId: string, unitId: string): string {
  return `novel/${bookId}/units/${unitId}.md`
}

export function novelUnitReviewPath(bookId: string, unitId: string): string {
  return `novel/${bookId}/reviews/${unitId}-review.md`
}

export function novelUnitsDir(bookId: string): string {
  return `novel/${bookId}/units`
}

export function novelUnitOutlinesDir(bookId: string): string {
  return `novel/${bookId}/outline/units`
}

export function buildUnitEntries(outlineNodes: NovelFileNode[], proseNodes: NovelFileNode[]): NovelUnitEntry[] {
  const byId = new Map<string, NovelUnitEntry>()
  const ensure = (name: string): NovelUnitEntry | null => {
    const meta = unitMetaFromName(name)
    if (!meta) return null
    let e = byId.get(meta.unitId)
    if (!e) {
      e = {
        unitId: meta.unitId,
        volume: meta.volume,
        index: meta.index,
        label: meta.unitId,
        chapterFrom: 0,
        chapterTo: 0,
        outline: null,
        prose: null,
      }
      byId.set(meta.unitId, e)
    }
    return e
  }
  for (const node of outlineNodes) {
    if (node.isDir || !isNovelUnitOutlineName(node.name)) continue
    const e = ensure(node.name)
    if (e && !e.outline) e.outline = node
  }
  for (const node of proseNodes) {
    if (node.isDir || !isNovelUnitProseName(node.name)) continue
    const e = ensure(node.name)
    if (e && !e.prose) e.prose = node
  }
  return [...byId.values()].sort(compareUnits)
}

function sectionBlocks(raw: string, section: string): string[][] {
  const lines = raw.split(/\r?\n/)
  let on = false
  const blocks: string[][] = []
  let cur: string[] | null = null
  const flush = () => {
    if (cur && cur.length) blocks.push(cur)
    cur = null
  }
  for (const line of lines) {
    if (!on) {
      if (line === `${section}:` || line.startsWith(`${section}:`)) on = true
      continue
    }
    if (/^[A-Za-z_][\w]*:/.test(line)) {
      flush()
      break
    }
    if (/^\s+-\s+/.test(line)) {
      flush()
      cur = [line]
      continue
    }
    if (cur) cur.push(line)
  }
  flush()
  return blocks
}

function blockField(block: string[], key: string): string {
  const re = new RegExp(`(?:^|\\s)${key}:\\s*(.*)$`)
  for (const line of block) {
    const m = line.match(re)
    if (m) return m[1].replace(/^["']|["']$/g, '').trim()
  }
  return ''
}

export function parseUnitOutlineYaml(raw: string): NovelUnitOutlineFields {
  const scenes: NovelUnitScene[] = sectionBlocks(raw, 'scenes').map((block) => ({
    id: blockField(block, 'id'),
    beat: blockField(block, 'beat'),
    chapter: Number.parseInt(blockField(block, 'chapter'), 10) || 0,
  }))
  const chapters: NovelUnitChapterCut[] = sectionBlocks(raw, 'chapters').map((block) => ({
    chapter: Number.parseInt(blockField(block, 'chapter'), 10) || 0,
    title: blockField(block, 'title_working'),
    cutHook: blockField(block, 'cut_hook'),
    wordShare: blockField(block, 'word_share'),
  }))
  return {
    unitId: yamlScalar(raw, 'unit_id'),
    status: yamlScalar(raw, 'status').toLowerCase() || 'proposed',
    title: yamlScalar(raw, 'title_working'),
    functionText: yamlScalar(raw, 'function'),
    desire: yamlScalar(raw, 'desire'),
    obstacle: yamlScalar(raw, 'obstacle'),
    pleasure: yamlScalar(raw, 'pleasure'),
    wordTarget: yamlScalar(raw, 'word_target'),
    hookType: yamlNestedScalar(raw, 'next_hook', 'type'),
    hookOut: yamlNestedScalar(raw, 'next_hook', 'out'),
    scenes,
    chapters,
  }
}

export function applyUnitOutline(entry: NovelUnitEntry, raw: string): NovelUnitEntry {
  const parsed = parseUnitOutlineYaml(raw)
  const from = parsed.chapters[0]?.chapter ?? 0
  const to = parsed.chapters.length ? parsed.chapters[parsed.chapters.length - 1].chapter : 0
  const range = raw.match(/chapter_range:\s*\[(\d+)\s*,\s*(\d+)\]/)
  return {
    ...entry,
    chapterFrom: range ? Number(range[1]) : from,
    chapterTo: range ? Number(range[2]) : to,
    label: parsed.title ? `${entry.unitId} · ${parsed.title}` : entry.unitId,
  }
}

export function inferUnitPhase(
  entry: NovelUnitEntry,
  lastCommittedCh: number,
  outlineRaw?: string,
  reviewRaw?: string,
): NovelChapterPhase {
  const status = outlineRaw ? parseUnitOutlineYaml(outlineRaw).status : ''
  const verdict = reviewRaw ? parseReviewVerdict(reviewRaw) : null
  const covered =
    entry.prose && entry.chapterTo > 0 && entry.chapterTo <= lastCommittedCh
  if (covered) return 'committed'
  if (entry.prose) {
    if (verdict === 'FAIL') return 'review_fail'
    if (verdict === 'PASS' || status === 'reviewed') return 'review_pass'
    return 'drafted'
  }
  if (entry.outline) {
    if (status === 'accepted' || status === 'drafted' || status === 'reviewed') return 'contract_ready'
    return 'contract_draft'
  }
  return 'empty'
}

export function buildUnitPhases(
  entries: NovelUnitEntry[],
  lastCommittedCh: number,
  outlineRaws: Record<string, string> = {},
  reviewRaws: Record<string, string> = {},
): Record<string, NovelChapterPhase> {
  const out: Record<string, NovelChapterPhase> = {}
  for (const e of entries) {
    const applied = outlineRaws[e.unitId] ? applyUnitOutline(e, outlineRaws[e.unitId]) : e
    out[e.unitId] = inferUnitPhase(applied, lastCommittedCh, outlineRaws[e.unitId], reviewRaws[e.unitId])
  }
  return out
}

export function splitUnitChapters(md: string): { chapter: number; title: string }[] {
  return splitUnitProseSections(md).map((s) => ({ chapter: s.chapter, title: s.title }))
}

/** Split one unit file on `## 第N章`. A lone `---` is a chapter cut, not a scene break. */
export function splitUnitProseSections(md: string): NovelUnitProseSection[] {
  const lines = md.replace(/\r\n/g, '\n').split('\n')
  const sections: { chapter: number; title: string; lines: string[] }[] = []
  let cur: { chapter: number; title: string; lines: string[] } | null = null
  for (const line of lines) {
    const m = line.match(/^## 第(\d+)章(?:\s+(.*))?$/)
    if (m) {
      if (cur) sections.push(cur)
      cur = { chapter: Number(m[1]), title: (m[2] || '').trim(), lines: [] }
      continue
    }
    if (!cur || line.trim() === '---') continue
    cur.lines.push(line)
  }
  if (cur) sections.push(cur)
  return sections.map((s) => ({
    chapter: s.chapter,
    title: s.title,
    body: s.lines.join('\n').trim(),
  }))
}

export function countPlainChars(text: string): number {
  return text.replace(/\s+/g, '').length
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

/** Merge chapters/*.md prose and chapter-outline YAML under chapters/ (`*-outline.yaml`). */
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
  const root = `novel/${bookId}`
  const vol = ctx.volume && ctx.volume > 0 ? ctx.volume : 0
  const volPad = vol > 0 ? String(vol).padStart(2, '0') : 'NN'
  const unitId = (ctx.unitId ?? '').trim() || 'vNN-U#'
  const unitPath = ctx.unitPath || `${root}/units/${unitId}.md`
  const outlinePath = `${root}/outline/units/${unitId}.yaml`
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
        `产出总纲 ${root}/outline/book_outline.md（不写单元正文）。`,
        '卷纲另做；场面和章切口只进单元细纲。',
      ].join('\n')
    case 'volume':
      return [
        `为第 ${vol || 'N'} 卷写卷纲 → ${root}/outline/volumes/v${volPad}.md`,
        '止于剧情单元卡；不写单元细纲/正文。写完等我确认（本卷点名人物一并 canon）。',
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
        `为单元 ${unitId} 写单元细纲 → ${outlinePath}`,
        '从本卷纲单元卡下推；场面覆盖章范围，每章至少 2 场。就绪后 status=accepted，并写入 active_unit。',
      ].join('\n')
    case 'write':
      return [
        `写单元 ${unitId} 正文首稿，一份文件 ${unitPath}。`,
        '章与章用单独一行 --- 分隔，标题为 ## 第N章。先 gate preflight --unit，只消费 ### CONTEXT + 本单元细纲；落盘后停下。',
        '本轮不要扩写、去 AI 味或 Continuity Commit（另开一轮定稿，便于换模）。一轮只写这一个单元。',
      ].join('\n')
    case 'continue':
      return [
        `接着写下个单元的细纲和正文首稿（书：${root}/）。`,
        'gate preflight --unit → 一份 units/vNN-U#.md 后停下；扩写/润色/定稿另开一轮。',
      ].join('\n')
    case 'expand':
      return [
        `对 ${unitPath} 做字数/厚度扩写并落盘（扩场面，不按章注水）。`,
        '按 expansion 纪律 ≤3 种技术；改完复跑 gate precommit --unit。保持章标题和 ---。',
      ].join('\n')
    case 'review':
      return [
        `审阅单元 ${unitId}（${unitPath}）。`,
        '先 gate precommit --unit。PASS：只更新 gates.qc，不写 review 文件。FAIL/深审：写 reviews/' + unitId + '-review.md。',
      ].join('\n')
    case 'polish':
      return [
        `对 ${unitPath} 做去 AI 味润色并落盘。`,
        'gate scan-deslop --unit 定位后按行号改；不改情节 Canon。',
      ].join('\n')
    case 'commit':
      return [
        `对单元 ${unitId}（${unitPath}）做 Continuity Commit。`,
        '一次补丁更新 ledger 里该单元每一章 ## chNNN + 细纲 reviewed + last_committed_ch；再 gate postcommit --unit。',
      ].join('\n')
    case 'review-polish-commit':
      return [
        `对单元 ${unitId}（${unitPath}）定稿串行：扩写(如需) → 审 → 可选润色 → 一次 Commit。`,
        '这是写作之后的定稿轮（可换经济/质检模型）。PASS 不写 review 文件；FAIL 才落盘。不要按章拆成多次 Commit。',
      ].join('\n')
    case 'continuation':
      return [
        `续写/接手本书（${root}/）。`,
        'Frozen_Canon 未确认禁止写正文。旧 chapters/ 需迁成 units/ 后再写。',
      ].join('\n')
    case 'preflight':
      return [
        `写前预检（书：${root}/，单元 ${unitId}）。`,
        'exec_shell gate preflight --unit；回报 ### CONTEXT 与 VERDICT。',
      ].join('\n')
    default:
      return ''
  }
}
