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

export interface NovelLoadProtocol {
  skillId: NovelSkillId
  skillRefs: string[]
  kbThemes: string[]
  readFiles: string[]
}

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

function novelPrefillRoot(ctx: NovelStagePrefillCtx): string {
  const bookId = (ctx.bookId ?? '').trim() || '<book-id>'
  return `novel/${bookId}`
}

function novelPrefillChPad(ctx: NovelStagePrefillCtx): string {
  const ch = ctx.chapter && ctx.chapter > 0 ? ctx.chapter : 0
  return ch > 0 ? String(ch).padStart(3, '0') : 'NNN'
}

function novelPrefillVolPad(ctx: NovelStagePrefillCtx): string {
  const vol = ctx.volume && ctx.volume > 0 ? ctx.volume : 0
  return vol > 0 ? String(vol).padStart(2, '0') : 'NN'
}

export function novelActionLoadProtocol(action: NovelStageAction, ctx: NovelStagePrefillCtx): NovelLoadProtocol {
  const root = novelPrefillRoot(ctx)
  const chPad = novelPrefillChPad(ctx)
  const volPad = novelPrefillVolPad(ctx)
  const state = `${root}/novel-state.yaml`
  const bibleHint = `${root}/book-bible.md（只读终局储备三行，禁止整本）`
  const ledger = `${root}/continuity/ledger.md（尾部：facts / tracking / loops / 近 3 章摘要）`
  const contract = `${root}/chapters/ch${chPad}-contract.yaml`
  const prose = ctx.chapterPath || `${root}/chapters/ch${chPad}.md`
  const volume = `${root}/outline/volumes/v${volPad}.md`
  const bookOutline = `${root}/outline/book_outline.md`
  const cast = `${root}/canon/cast/（上场人物卡，按合同登场名单）`
  const world = `${root}/canon/world.md`

  switch (action) {
    case 'init':
      return {
        skillId: 'novel-setup',
        skillRefs: ['novel-setup/references/init.md', 'novel-setup/references/project-layout.md'],
        kbThemes: ['题材与平台'],
        readFiles: [],
      }
    case 'assets':
      return {
        skillId: 'novel-plan',
        skillRefs: [
          'novel-setup/assets/templates/world.md',
          'novel-setup/assets/templates/cast-card.md',
        ],
        kbThemes: ['人设与群像'],
        readFiles: [state, bibleHint, world, `${root}/canon/cast/`],
      }
    case 'goldfinger':
      return {
        skillId: 'novel-plan',
        skillRefs: ['novel-setup/assets/templates/cast-card.md'],
        kbThemes: ['世界观与金手指'],
        readFiles: [state, bibleHint, world, `${root}/canon/cast/`],
      }
    case 'outline':
      return {
        skillId: 'novel-plan',
        skillRefs: [
          'novel-plan/references/outline.md',
          'novel-plan/assets/templates/book-outline.md',
        ],
        kbThemes: ['节奏与结构'],
        readFiles: [state, bibleHint, world],
      }
    case 'volume':
      return {
        skillId: 'novel-plan',
        skillRefs: ['novel-plan/references/outline.md', 'novel-plan/assets/templates/volume-outline.md'],
        kbThemes: ['节奏与结构'],
        readFiles: [state, bookOutline, volume, ledger],
      }
    case 'contract':
      return {
        skillId: 'novel-write',
        skillRefs: [
          'novel-write/references/chapter-contract.md',
          'novel-write/assets/templates/chapter-contract.yaml',
        ],
        kbThemes: ['节奏与结构'],
        readFiles: [state, `${root}/outline/volumes/（本卷纲：定位 unit_id）`, bibleHint],
      }
    case 'write':
      return {
        skillId: 'novel-write',
        skillRefs: [
          'novel-write/references/preflight.md',
          'novel-write/references/chapter-write.md',
        ],
        kbThemes: ['文风与去 AI 味'],
        readFiles: [state, contract, ledger, cast],
      }
    case 'continue':
      return {
        skillId: 'novel-write',
        skillRefs: [
          'novel-write/references/continuation.md',
          'novel-write/references/chapter-write.md',
        ],
        kbThemes: ['文风与去 AI 味'],
        readFiles: [state, ledger],
      }
    case 'dialogue':
      return {
        skillId: 'novel-write',
        skillRefs: ['novel-write/references/scene-routing.md', 'novel-write/references/chapter-write.md'],
        kbThemes: ['情绪与场景'],
        readFiles: [state, contract, prose, cast],
      }
    case 'hook':
      return {
        skillId: 'novel-write',
        skillRefs: ['novel-write/references/chapter-write.md'],
        kbThemes: ['爽点与追读'],
        readFiles: [state, contract, prose],
      }
    case 'reversal':
      return {
        skillId: 'novel-write',
        skillRefs: ['novel-write/references/scene-routing.md'],
        kbThemes: ['爽点与追读'],
        readFiles: [state, contract, prose],
      }
    case 'preflight':
      return {
        skillId: 'novel-write',
        skillRefs: ['novel-write/references/preflight.md'],
        kbThemes: [],
        readFiles: [state, contract, ledger, cast],
      }
    case 'batch-freeze':
      return {
        skillId: 'novel-write',
        skillRefs: ['novel-write/references/batch-freeze.md'],
        kbThemes: [],
        readFiles: [state, `${root}/outline/volumes/（本卷纲）`, `${root}/chapters/（该批章合同）`, ledger],
      }
    case 'continuation':
      return {
        skillId: 'novel-write',
        skillRefs: [
          'novel-write/references/continuation.md',
          'novel-write/assets/templates/style-fingerprint.md',
        ],
        kbThemes: ['文风与去 AI 味'],
        readFiles: [state, ledger],
      }
    case 'review':
      return {
        skillId: 'novel-review',
        skillRefs: ['novel-review/references/review-gates.md'],
        kbThemes: ['文风与去 AI 味'],
        readFiles: [prose, contract],
      }
    case 'polish':
      return {
        skillId: 'novel-review',
        skillRefs: [
          'novel-review/references/polish-deslop.md',
          'novel-setup/references/gate.md',
        ],
        kbThemes: ['文风与去 AI 味'],
        readFiles: [prose],
      }
    case 'commit':
      return {
        skillId: 'novel-review',
        skillRefs: ['novel-review/references/continuity-commit.md'],
        kbThemes: [],
        readFiles: [prose, ledger, state],
      }
    case 'review-polish-commit':
      return {
        skillId: 'novel-review',
        skillRefs: [
          'novel-review/references/review-gates.md',
          'novel-review/references/polish-deslop.md',
          'novel-review/references/continuity-commit.md',
          'novel-setup/references/gate.md',
        ],
        kbThemes: ['文风与去 AI 味'],
        readFiles: [prose, contract, state, ledger],
      }
    case 'batch-review':
      return {
        skillId: 'novel-review',
        skillRefs: ['novel-review/references/review-gates.md'],
        kbThemes: ['文风与去 AI 味'],
        readFiles: [`${root}/chapters/（最近 1–5 章有正文未 PASS）`, `${root}/reviews/`],
      }
    default:
      return { skillId: 'novel-write', skillRefs: [], kbThemes: [], readFiles: [state] }
  }
}

export function formatLoadProtocol(protocol: NovelLoadProtocol): string {
  const refs = protocol.skillRefs.length
    ? ` 以及 ${protocol.skillRefs.join('、')}`
    : ''
  const lines = [
    `【本轮技能】${protocol.skillId}（Composer 已勾选；先 read_skill）`,
    '【加载顺序 — 未完成禁止 write/edit】',
    `1. read_skill ${protocol.skillId}${refs}`,
  ]
  if (protocol.kbThemes.length) {
    lines.push(`2. search_kb ${protocol.kbThemes.join('；')}`)
  } else {
    lines.push('2. search_kb：本阶段无需检索技法库')
  }
  lines.push('3. read_file 下列路径（按序；禁止整本 bible / 全卷纲）：')
  if (protocol.readFiles.length) {
    for (const file of protocol.readFiles) lines.push(`   - ${file}`)
  } else {
    lines.push('   - （无必读项目文件）')
  }
  return lines.join('\n')
}

export function buildConstraintFooter(
  pipeline: NovelBookPipeline,
  action: NovelStageAction,
  blockers: string[],
): string {
  const skillId = novelActionSkillId(action)
  const lines = [
    '---',
    '【工作台约束 — 必须遵守】',
    `- 当前书阶段：${pipeline.phase}（步进：${pipeline.step}）；请求动作：${action}`,
    `- 本轮只准用技能 ${skillId}；未 read_skill 成功前禁止 write/edit。`,
    `- knowledge_gate：${pipeline.gates.knowledge} | asset_gate：${pipeline.gates.asset} | qc_gate：${pipeline.gates.qc}`,
  ]
  if (blockers.length) lines.push(`- 阻断项：${blockers.join('；')}`)
  lines.push('- 若门禁未 PASS，禁止写正文/Commit；用 ask_user 说明阻断项。')
  lines.push('- 完成后更新 novel-state.yaml 的 gates/blockers 字段。')
  lines.push('- 换阶段时请在 Composer 自行切换模型；工作台不会自动换模。')
  lines.push('- Team：delegate_agent.goal 必须包含本消息任务正文原文，禁止改写。')
  return lines.join('\n')
}

export function buildConstrainedPrefill(
  action: NovelStageAction,
  ctx: NovelStagePrefillCtx,
  pipeline?: NovelBookPipeline,
  blockers?: string[],
): string {
  const protocol = formatLoadProtocol(novelActionLoadProtocol(action, ctx))
  const body = buildNovelStagePrefill(action, ctx)
  const parts = [protocol, `【任务】\n${body}`]
  if (pipeline) {
    parts.push(buildConstraintFooter(pipeline, action, blockers ?? []))
  } else {
    parts.push(
      [
        '---',
        '【工作台约束 — 必须遵守】',
        `- 本轮只准用技能 ${novelActionSkillId(action)}；未 read_skill 成功前禁止 write/edit。`,
        '- Team：delegate_agent.goal 必须包含本消息任务正文原文，禁止改写。',
      ].join('\n'),
    )
  }
  return parts.join('\n\n')
}

export const NOVEL_PIPELINE_STEPS: { id: NovelPipelineStepId; action?: NovelStageAction }[] = [
  { id: 'init', action: 'init' },
  { id: 'setup', action: 'assets' },
  { id: 'outline', action: 'outline' },
  { id: 'volume', action: 'volume' },
  { id: 'contract', action: 'contract' },
  { id: 'write', action: 'write' },
  { id: 'review', action: 'review' },
  { id: 'commit', action: 'commit' },
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
  // Keep intent + paths only. Field lists / section inventories live in skill
  // refs from novelActionLoadProtocol — do not restate them here.

  switch (action) {
    case 'init':
      return [
        '开一本新书并立项。',
        '用 ask_user 澄清题材、读者承诺、篇幅/平台、禁忌（一次一问即可）。',
        `落盘标准树到 novel/<book-id>/（布局见加载协议中的 project-layout）。`,
        '新角色先 candidate，经确认后再 canon；确认前不要写正文。',
      ].join('\n')
    case 'outline':
      return [
        `书目录：${root}/`,
        `产出总纲 ${root}/outline/book_outline.md 与卷纲 ${root}/outline/volumes/vNN.md（不写章节正文）。`,
        '卷纲止于剧情单元卡；单章任务/爽点/钩子只进章合同。每卷写完等我确认。',
      ].join('\n')
    case 'volume':
      return [
        `为第 ${vol || 'N'} 卷写卷纲。`,
        `唯一落盘：${root}/outline/volumes/v${volPad}.md`,
        '先读总纲、相关 canon、未回收伏笔；与前后卷衔接。',
        '止于剧情单元卡（含节拍覆盖章范围）；不写章合同/正文。写完等我确认。',
      ].join('\n')
    case 'assets':
      return [
        `书目录：${root}/`,
        '整理/补全人物卡与世界观：写入 canon/ 与 table_*（带 book_id）。',
        '新实体先 candidate，经确认后再 canon；确认前不要写正文。',
      ].join('\n')
    case 'goldfinger':
      return [
        `书目录：${root}/`,
        '设计或修订金手指，默认写入主角 cast 卡（不必单建 goldfinger.md）。',
        '先 candidate，经确认后再 canon；未确认时不要改已定稿正文。',
      ].join('\n')
    case 'contract':
      return [
        `为第 ${ch || 'N'} 章写章合同（尚不写正文）。`,
        `落盘：${root}/chapters/ch${chPad}-contract.yaml`,
        '从本卷纲所属单元卡按节拍下推；必填 unit_id（vNN-U#）。对不上或节拍未覆盖本章 → 先补卷纲，勿空造合同。',
        '里程碑章或本批首章需 ask_user 接受后再写正文。',
      ].join('\n')
    case 'write':
      return [
        `写第 ${ch || 'N'} 章正文到 ${chPath}。`,
        '前提：该章合同已 accepted（否则先补合同）。',
        '按合同草稿落盘；P0 审稿不过不得定稿；不要只在对话里贴正文。',
      ].join('\n')
    case 'continue':
      return [
        `接着写下一章（书：${root}/）。`,
        '补/更新章合同后再写正文；审一轮，P0 不过不得定稿；Commit 落盘。不要只在对话里贴正文。',
      ].join('\n')
    case 'dialogue':
      return [
        `加强第 ${ch || 'N'} 章对话（${chPath}）。`,
        '按合同 beats 写出可辨声口、推动冲突的对白；落盘到该章正文。',
      ].join('\n')
    case 'hook':
      return [
        `为第 ${ch || 'N'} 章做爽点强化：设计/改写章末悬念钩（${chPath}）。`,
        '章末 hook 须可执行；写入合同并落到正文。',
      ].join('\n')
    case 'reversal':
      return [
        `为第 ${ch || 'N'} 章做爽点强化：加一处反转（${chPath}）。`,
        '须服务合同 purpose，不推翻 Frozen_Canon；先改合同再改正文。',
      ].join('\n')
    case 'review':
      return [
        `审阅 ${chPath}，审查报告写入 ${root}/reviews/。`,
        'PASS 写短 stub（含追更指数）；FAIL 写全文六镜。qc_gate FAIL 不得宣称定稿。',
      ].join('\n')
    case 'polish':
      return [
        `对 ${chPath} 做去 AI 味润色并落盘。`,
        '先 exec_shell gate --action scan-deslop --chapter N，按 HITS 行号 edit；再清 P1。',
        '改完再扫一遍（或 precommit）；exit 0 才可宣称去 AI 味。不改情节 Canon；需改情节则回到审稿/章合同。',
      ].join('\n')
    case 'commit':
      return [
        ch > 0
          ? `对第 ${ch} 章（${chPath}）做 Continuity Commit。`
          : `对当前进度做 Continuity Commit（书：${root}/）。`,
        '一次补丁更新 continuity/ledger.md + 合同 status + novel-state；完成=工具证据。',
      ].join('\n')
    case 'review-polish-commit':
      return [
        ch > 0
          ? `对第 ${ch} 章（${chPath}）串行：审稿 → 去 AI 味 → Continuity Commit。`
          : `对当前章节串行：审稿 → 去 AI 味 → Continuity Commit（书：${root}/）。`,
        '未 PASS 禁止润色与定稿。润色段先 scan-deslop 再按行号改。若 reviews/ 已有 PASS 且正文未变，可从润色起并注明跳过理由。完成=工具证据。',
      ].join('\n')
    case 'batch-freeze':
      const bFrom = ctx.batchFrom && ctx.batchFrom > 0 ? ctx.batchFrom : 1
      const bTo = ctx.batchTo && ctx.batchTo > 0 ? ctx.batchTo : 8
      return [
        `批次冻结（书：${root}/，第 ${bFrom}–${bTo} 章）。`,
        '该批章合同须 accepted，且每章 unit_id 指向本卷剧情单元。',
        '只更新 novel-state.yaml 的 frozen_batch / artifacts.batch_freeze（不要写 batch-freeze.yaml）。冻结前禁止批量写正文。',
      ].join('\n')
    case 'continuation':
      return [
        `续写/接手本书（${root}/）。`,
        'Frozen_Canon 未确认禁止写正文；首次续写短试写待确认。',
      ].join('\n')
    case 'batch-review':
      return [
        `批量审稿（书：${root}/）：最近 1–5 章有正文但未 PASS 的章节。`,
        '每章独立 review 文件；PASS 用短 stub；更新 gates.qc 与 blockers。',
      ].join('\n')
    case 'preflight':
      return [
        `写前预检（书：${root}/）。`,
        '按加载协议读必读文件；把一行回执写入 novel-state.last_preflight；更新 gates/blockers。阻断则停止并 ask_user。',
      ].join('\n')
    default:
      return ''
  }
}
