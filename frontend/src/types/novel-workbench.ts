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
  genre: string
  qcProfile: string
  craftLane: string
  continuationMode: boolean
  activeUnit: string
  /** artifacts.cast_registry — `ok` / `fail` written by gate cast-lint, else raw value. */
  castRegistry: string
  gates: { knowledge: GateStatus; asset: GateStatus; qc: GateStatus }
  blockers: string[]
}

/**
 * Three workbench phases (+ idle):
 * planning  — no approved volume outline yet (or approved but not accept-volume'd)
 * outlining — the current volume still has `proposed` unit YAML heads
 * units     — every unit of the current volume is outlined; write → finalize one by one
 */
export type NovelPipelinePhase = 'planning' | 'outlining' | 'units' | 'idle'

/**
 * Unit state inferred from disk:
 * pending_outline — `proposed` head without scenes (seeded by accept-volume)
 * ready           — `accepted` outline, no prose
 * drafted         — prose exists, not yet finalized
 * review_fail     — prose exists and reviews/<unit>-review.md says FAIL
 * finalized       — `reviewed` and chapter range ≤ last_committed_ch
 */
export type NovelUnitPhase = 'pending_outline' | 'ready' | 'drafted' | 'review_fail' | 'finalized'

/** Primary actions drive the single CTA; secondary actions live under 「更多」. */
export type NovelPrimaryAction = 'plan' | 'outline-batch' | 'write' | 'finalize' | 'next-volume'
export type NovelSecondaryAction = 'contract-one' | 'expand' | 'review' | 'polish' | 'cast-fix'
export type NovelStageAction = 'init' | 'migrate' | NovelPrimaryAction | NovelSecondaryAction

export const NOVEL_PRIMARY_ACTIONS: readonly NovelPrimaryAction[] = [
  'plan',
  'outline-batch',
  'write',
  'finalize',
  'next-volume',
]

export const NOVEL_SECONDARY_ACTIONS: readonly NovelSecondaryAction[] = [
  'contract-one',
  'expand',
  'review',
  'polish',
  'cast-fix',
]

export type NovelSkillId = 'novel-setup' | 'novel-plan' | 'novel-write' | 'novel-review'

export interface NovelStagePrefillCtx {
  bookId?: string
  unitId?: string
  unitPath?: string
  volume?: number
  /** For outline-batch: the ≤4 proposed unit ids in this round. */
  batchUnits?: string[]
  /** For cast-fix: the cast stem to complete. */
  stem?: string
  /** True when the volume outline file already exists (plan → approve + accept-volume). */
  volumeOutlineExists?: boolean
}

// ---------------------------------------------------------------------------
// YAML scalars (no dependency)
// ---------------------------------------------------------------------------

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
    if (v.startsWith('#')) v = ''
  }
  if (v === '""' || v === "''") return ''
  return v
}

function yamlNestedScalar(raw: string, parent: string, key: string): string {
  // Capture indented block under `parent:` until next top-level key.
  const blockRe = new RegExp(`^${parent}:\\s*\\n((?:[ \\t].*\\n?)*)`, 'm')
  const block = raw.match(blockRe)
  if (!block) return ''
  const re = new RegExp(`^[ \\t]+${key}:\\s*(.*)$`, 'm')
  const m = block[1].match(re)
  if (!m) return ''
  let v = (m[1] ?? '').trim()
  const hash = v.search(/\s+#/)
  if (hash >= 0 && !/^["']/.test(v)) v = v.slice(0, hash).trim()
  return v.replace(/^["']|["']$/g, '')
}

/** `key: [a, b]` flow list or `key:\n  - a\n  - b` block list → string[]. */
function yamlList(raw: string, key: string): string[] {
  const flow = raw.match(new RegExp(`^${key}:\\s*\\[([^\\]]*)\\]`, 'm'))
  if (flow) {
    return flow[1]
      .split(',')
      .map((s) => s.trim().replace(/^["']|["']$/g, ''))
      .filter(Boolean)
  }
  const block = raw.match(new RegExp(`^${key}:\\s*\\n((?:[ \\t]+-\\s+.*\\n?)*)`, 'm'))
  if (!block) return []
  const out: string[] = []
  for (const line of block[1].split('\n')) {
    const m = line.match(/^\s+-\s+(.*)$/)
    if (!m) continue
    let v = m[1].trim()
    const hash = v.search(/\s+#/)
    if (hash >= 0 && !/^["']/.test(v)) v = v.slice(0, hash).trim()
    v = v.replace(/^["']|["']$/g, '')
    if (v) out.push(v)
  }
  return out
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

  const blockers: string[] = []
  const blockerBlock = raw.match(/^blockers:\s*\n((?:\s+-\s+.+\n?)*)/m)
  if (blockerBlock) {
    for (const line of blockerBlock[1].split('\n')) {
      const m = line.match(/^\s*-\s+(.*)$/)
      if (m?.[1]?.trim()) blockers.push(m[1].trim().replace(/^["']|["']$/g, ''))
    }
  }

  return {
    ...base,
    genre: yamlScalar(raw, 'genre'),
    qcProfile: yamlScalar(raw, 'qc_profile') || 'general',
    craftLane: yamlScalar(raw, 'craft_lane') || 'default',
    continuationMode: /continuation_mode:\s*true/i.test(raw),
    activeUnit: yamlScalar(raw, 'active_unit'),
    castRegistry: yamlNestedScalar(raw, 'artifacts', 'cast_registry'),
    gates: parseGateBlock(raw),
    blockers,
  }
}

export function parseReviewVerdict(raw: string): 'PASS' | 'FAIL' | null {
  const m = raw.match(/###\s*VERDICT\s*\n\s*(PASS|FAIL)/i)
  if (!m) return null
  return m[1].toUpperCase() === 'PASS' ? 'PASS' : 'FAIL'
}

// ---------------------------------------------------------------------------
// Paths
// ---------------------------------------------------------------------------

export function novelActiveBookPath(): string {
  return 'novel/.active-book'
}

export function novelBookDir(bookId: string): string {
  return `novel/${bookId}`
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

export function novelFactsPath(bookId: string): string {
  return `novel/${bookId}/continuity/facts.md`
}

export function novelSummariesDir(bookId: string): string {
  return `novel/${bookId}/continuity/summaries`
}

export function novelOutlineDir(bookId: string): string {
  return `novel/${bookId}/outline`
}

export function novelVolumesDir(bookId: string): string {
  return `novel/${bookId}/outline/volumes`
}

export function novelVolumeOutlinePath(bookId: string, volume: number): string {
  return `novel/${bookId}/outline/volumes/${volumeId(volume)}.md`
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

export function novelCastCardPath(bookId: string, stem: string): string {
  return `novel/${bookId}/canon/cast/${stem}.md`
}

export function novelLegacyChaptersDir(bookId: string): string {
  return `novel/${bookId}/chapters`
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

/** `v01` for 1. */
export function volumeId(volume: number): string {
  return `v${String(Math.max(0, volume)).padStart(2, '0')}`
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
    'author-lore.md': 'authorLore',
    'style-fingerprint.md': 'style',
    'locked-terms.yaml': 'lockedTerms',
  }
  if (known[key]) return known[key]
  return name.replace(/^\d+-/, '').replace(/\.(md|ya?ml)$/i, '')
}

export function sortMdNodes(nodes: NovelFileNode[]): NovelFileNode[] {
  return nodes
    .filter((n) => !n.isDir && /\.md$/i.test(n.name))
    .slice()
    .sort((a, b) => a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: 'base' }))
}

/** Outline / misc docs: markdown + yaml. */
export function sortWorkbenchDocNodes(nodes: NovelFileNode[]): NovelFileNode[] {
  return nodes
    .filter((n) => !n.isDir && /\.(md|ya?ml)$/i.test(n.name))
    .slice()
    .sort((a, b) => a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: 'base' }))
}

export function chapterNumFromName(name: string): number | null {
  const m = name.match(/(\d+)/)
  if (!m) return null
  const n = Number.parseInt(m[1], 10)
  return Number.isFinite(n) ? n : null
}

// ---------------------------------------------------------------------------
// Markdown tables (book outline / volume index / cast relations)
// ---------------------------------------------------------------------------

export interface BookOutlineVolumeRow {
  vol: string
  goal: string
  climax: string
  twist: string
}

/** One row of the volume 单元索引 table (or a legacy unit card / table). */
export interface VolumeUnitRow {
  id: string
  range: string
  purpose: string
  endgameBoundary: string
  hookType: string
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

export function parseChapterRange(s: string): { from: number; to: number } | null {
  const m = s.match(/ch?\s*(\d+)\s*[-–—~至到]\s*(?:ch)?\s*(\d+)/i) || s.match(/(\d+)\s*[-–—~至到]\s*(\d+)/)
  if (!m) return null
  const from = Number.parseInt(m[1], 10)
  const to = Number.parseInt(m[2], 10)
  if (!Number.isFinite(from) || !Number.isFinite(to)) return null
  return { from: Math.min(from, to), to: Math.max(from, to) }
}

function stripMdInline(s: string): string {
  return s.replace(/^`+|`+$/g, '').trim()
}

const UNIT_ID_RE = /^v\d{2,}-U\d+$/i
const UNIT_ID_HEADERS = new Set(['unit_id', '单元', '单元 id', '单元id', 'unit'])

/**
 * Rows of the 单元索引 table:
 * `| unit_id | 章范围 | 一句话功能 | 本单元终局边界（禁碰） | 下一单元钩子类型 |`.
 * `U1` in the first column is expanded with `volume` (e.g. 1 → v01-U1).
 * Falls back to legacy `### 剧情单元` cards and older tables.
 */
export function parseVolumeUnitRows(md: string, volume?: number): VolumeUnitRow[] {
  const table = parseVolumeIndexTable(md, volume)
  if (table.length) return table
  const cards = parseVolumeUnitCards(md)
  if (cards.length) return cards
  return parseLegacyUnitTable(md)
}

function expandUnitId(first: string, volume?: number): string {
  const m = first.match(/^U(\d+)$/i)
  if (m && volume && volume > 0) return `${volumeId(volume)}-U${m[1]}`
  return first
}

function parseVolumeIndexTable(md: string, volume?: number): VolumeUnitRow[] {
  const rows: VolumeUnitRow[] = []
  let grab = false
  let sawTable = false
  for (const line of md.split(/\r?\n/)) {
    const s = line.trim()
    if (s.startsWith('#')) {
      if (grab && rows.length) break
      grab = s.includes('单元索引')
      sawTable = false
      continue
    }
    if (!grab) continue
    const cells = markdownTableCells(line)
    if (!cells.length) {
      if (sawTable && rows.length) break
      continue
    }
    sawTable = true
    if (isMarkdownSepRow(cells)) continue
    const first = stripMdInline(cells[0] ?? '')
    if (!first || UNIT_ID_HEADERS.has(first.toLowerCase())) continue
    const id = expandUnitId(first, volume)
    if (!UNIT_ID_RE.test(id)) continue
    rows.push({
      id,
      range: cells[1] ?? '',
      purpose: cells[2] ?? '',
      endgameBoundary: cells[3] ?? '',
      hookType: stripMdInline(cells[4] ?? ''),
    })
  }
  return rows
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
  const blank = (): VolumeUnitRow => ({ id: '', range: '', purpose: '', endgameBoundary: '', hookType: '' })

  for (const raw of md.split(/\r?\n/)) {
    const line = raw.trim()
    const heading = line.match(/^#{2,4}\s*剧情单元\s*(.+)?$/i)
    if (heading) {
      flush()
      cur = { ...blank(), id: (heading[1] || '').trim() }
      continue
    }
    const idLine = line.match(/^[-*]\s*单元ID[：:]\s*(.+)$/i)
    if (idLine) {
      if (!cur) cur = blank()
      cur.id = stripMdInline(idLine[1])
      continue
    }
    if (!cur) continue
    const rangeLine = line.match(/^[-*]\s*章范围[：:]\s*(.+)$/i)
    if (rangeLine) {
      cur.range = stripMdInline(rangeLine[1])
      continue
    }
    const purposeLine =
      line.match(/^[-*]\s*单元功能[（(][^）)]*[）)]?[：:]\s*(.+)$/i) ||
      line.match(/^[-*]\s*单元功能[：:]\s*(.+)$/i)
    if (purposeLine) {
      cur.purpose = stripMdInline(purposeLine[1])
      continue
    }
    if (/^##\s+/.test(line) && !/^##\s*剧情单元/i.test(line)) {
      flush()
      break
    }
  }
  flush()
  return rows.filter((r) => Boolean(r.id || r.range))
}

function parseLegacyUnitTable(md: string): VolumeUnitRow[] {
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
      rows.push({ id: cells[0], range: cells[1] ?? '', purpose: cells[2] ?? '', endgameBoundary: '', hookType: '' })
    } else {
      rows.push({ id: cells[0], range: cells[0], purpose: cells[1] ?? '', endgameBoundary: '', hookType: '' })
    }
  }
  return rows
}

/** Stems listed under `## 本卷人物` (one `- stem` per line; backticks / trailing notes tolerated). */
export function parseVolumeCast(md: string): string[] {
  const out: string[] = []
  let grab = false
  for (const line of md.split(/\r?\n/)) {
    const s = line.trim()
    if (s.startsWith('#')) {
      grab = s.includes('本卷人物')
      continue
    }
    if (!grab) continue
    const m = s.match(/^[-*]\s*`?([A-Za-z0-9][A-Za-z0-9_-]*)`?/)
    if (m && !out.includes(m[1])) out.push(m[1])
  }
  return out
}

// ---------------------------------------------------------------------------
// Cast cards
// ---------------------------------------------------------------------------

export type NovelCastStatus = 'candidate' | 'canon' | ''
export type NovelCastRole = 'protagonist' | 'volume_antagonist' | 'recurring' | ''

export interface NovelCastRelation {
  /** Counterpart stem. */
  target: string
  type: string
  current: string
  lastChange: string
  nextNode: string
}

export interface NovelCastCard {
  stem: string
  name: string
  status: NovelCastStatus
  role: NovelCastRole
  visualAnchor: string
  speechAnchor: string
  behaviorAnchor: string
  catchphrase: string
  desire: string
  intersect: string
  exit: string
  unknown: string
  dialogue: { pressure: string; daily: string; disguise: string }
  /** Human-facing labels of unfilled minimum fields for this role. */
  missing: string[]
  relations: NovelCastRelation[]
}

const CAST_KV_RE = /^\s*`?(status|role)`?\s*[：:]\s*([A-Za-z_-]+)/i
const CAST_ANCHOR_RE = /^\s*[-*]\s*\*?\*?(视觉|语言|行为)\*?\*?[：:]\s*(.*)$/
const CAST_UNKNOWN_RE = /^\s*[-*]\s*\*?\*?不知\*?\*?[：:]\s*(.*)$/
const CAST_DIALOGUE_RE = /^\s*[-*]\s*\*?\*?(压力下|日常|掩饰(?:\s*\/\s*说谎)?)\*?\*?[：:]\s*(.*)$/
const CAST_CATCHPHRASE_RE = /^\s*[-*]\s*\*?\*?口头禅[^：:]*[：:]\s*(.*)$/
const CAST_INTERSECT_RE = /^\s*[-*]\s*\*?\*?与主角相交点[^：:]*[：:]\s*(.*)$/
const CAST_DESIRE_RE = /^\s*[-*]\s*\*?\*?欲望[^：:]*[：:]\s*(.*)$/
const CAST_EXIT_RE = /^\s*[-*]\s*\*?\*?退场[^：:]*[：:]\s*(.*)$/
const TEMPLATE_PLACEHOLDER_RE = /^\{\{.*\}\}$/

function cleanCastValue(v: string): string {
  const s = v.trim()
  return TEMPLATE_PLACEHOLDER_RE.test(s) ? '' : s
}

export function castStemFromName(name: string): string {
  return name.replace(/\.md$/i, '')
}

/** Mirror of the gate's `parse_cast_card` (status / role header, anchors, samples, relation table). */
export function parseCastCard(md: string, stem: string): NovelCastCard {
  const lines = md.replace(/\r\n/g, '\n').split('\n')
  const first = (lines[0] ?? '').replace(/^#+\s*/, '').trim()
  const card: NovelCastCard = {
    stem,
    name: TEMPLATE_PLACEHOLDER_RE.test(first) ? '' : first,
    status: '',
    role: '',
    visualAnchor: '',
    speechAnchor: '',
    behaviorAnchor: '',
    catchphrase: '',
    desire: '',
    intersect: '',
    exit: '',
    unknown: '',
    dialogue: { pressure: '', daily: '', disguise: '' },
    missing: [],
    relations: [],
  }

  for (const line of lines.slice(0, 12)) {
    const m = line.match(CAST_KV_RE)
    if (!m) continue
    const key = m[1].toLowerCase()
    const val = m[2].trim().toLowerCase()
    if (key === 'status' && !card.status && (val === 'candidate' || val === 'canon')) card.status = val
    if (key === 'role' && !card.role && (val === 'protagonist' || val === 'volume_antagonist' || val === 'recurring')) {
      card.role = val
    }
  }

  let section = ''
  let relHeaderSeen = false
  for (const line of lines) {
    const s = line.trim()
    if (s.startsWith('#')) {
      section = s.replace(/^#+/, '').trim()
      relHeaderSeen = false
      continue
    }
    if (section.includes('三锚点')) {
      const m = line.match(CAST_ANCHOR_RE)
      if (m) {
        const v = cleanCastValue(m[2])
        if (m[1] === '视觉') card.visualAnchor = v
        else if (m[1] === '语言') card.speechAnchor = v
        else card.behaviorAnchor = v
        continue
      }
    }
    if (section.includes('台词')) {
      const m = line.match(CAST_DIALOGUE_RE)
      if (m) {
        const v = cleanCastValue(m[2])
        if (m[1] === '压力下') card.dialogue.pressure = v
        else if (m[1] === '日常') card.dialogue.daily = v
        else card.dialogue.disguise = v
        continue
      }
    }
    if (section.includes('知识边界')) {
      const m = line.match(CAST_UNKNOWN_RE)
      if (m) {
        card.unknown = cleanCastValue(m[1])
        continue
      }
    }
    if (section.includes('语言习惯')) {
      const m = line.match(CAST_CATCHPHRASE_RE)
      if (m) {
        card.catchphrase = cleanCastValue(m[1])
        continue
      }
    }
    if (section.includes('功能')) {
      const mi = line.match(CAST_INTERSECT_RE)
      if (mi) {
        card.intersect = cleanCastValue(mi[1])
        continue
      }
      const me = line.match(CAST_EXIT_RE)
      if (me) {
        card.exit = cleanCastValue(me[1])
        continue
      }
    }
    if (section.includes('四件套') && !card.desire) {
      const m = line.match(CAST_DESIRE_RE)
      if (m) {
        card.desire = cleanCastValue(m[1])
        continue
      }
    }
    if (section.includes('关系')) {
      const cells = markdownTableCells(line)
      if (!cells.length) continue
      if (isMarkdownSepRow(cells)) continue
      if (!relHeaderSeen) {
        relHeaderSeen = true
        if (cells[0] === '对方' || cells.includes('类型')) continue
      }
      const target = stripMdInline(cells[0] ?? '')
      if (!target) continue
      card.relations.push({
        target,
        type: cells[1] ?? '',
        current: cells[2] ?? '',
        lastChange: cells[3] ?? '',
        nextNode: cells[4] ?? '',
      })
    }
  }

  card.missing = castMissingFields(card)
  return card
}

function castDialogueSample(card: NovelCastCard): string {
  return card.dialogue.pressure || card.dialogue.daily || card.dialogue.disguise
}

/** Minimum fill per role (template 完整度 table); labels match the gate output. */
export function castMissingFields(card: NovelCastCard): string[] {
  const miss: string[] = []
  if (card.role === 'recurring') {
    if (!card.desire) miss.push('欲望')
    if (!card.intersect) miss.push('与主角相交点')
    if (!card.visualAnchor && !card.behaviorAnchor) miss.push('视觉锚或行为锚')
    if (!card.catchphrase) miss.push('口头禅')
    if (!castDialogueSample(card)) miss.push('台词样例')
  } else {
    if (!card.visualAnchor) miss.push('视觉锚')
    if (!card.speechAnchor) miss.push('语言锚')
    if (!card.behaviorAnchor) miss.push('行为锚')
    if (!card.unknown) miss.push('知识边界·不知')
    if (!card.dialogue.pressure) miss.push('台词·压力下')
    if (!card.dialogue.daily) miss.push('台词·日常')
    if (!card.dialogue.disguise) miss.push('台词·掩饰')
  }
  if (!card.exit) miss.push('退场')
  return miss
}

export interface NovelRelationEdge {
  from: string
  to: string
  type: string
  current: string
  /** True when `to` does not list `from` back (cast-lint blocking). */
  missingBackEdge: boolean
  /** True when `to` has no card at all. */
  unknownTarget: boolean
}

/** Relation edges across all cards; flags missing back edges and unknown stems like `cast-lint`. */
export function buildRelationEdges(cards: NovelCastCard[]): NovelRelationEdge[] {
  const byStem = new Map(cards.map((c) => [c.stem, c]))
  const edges: NovelRelationEdge[] = []
  for (const c of cards) {
    for (const r of c.relations) {
      const other = byStem.get(r.target)
      edges.push({
        from: c.stem,
        to: r.target,
        type: r.type,
        current: r.current,
        unknownTarget: !other,
        missingBackEdge: Boolean(other) && !other!.relations.some((x) => x.target === c.stem),
      })
    }
  }
  return edges
}

/** Local cast lint verdict from parsed cards (same rules the gate enforces). */
export function castLintIssues(cards: NovelCastCard[]): string[] {
  const issues: string[] = []
  if (!cards.length) return ['blocker.noCast']
  if (!cards.some((c) => c.role === 'protagonist' && c.status === 'canon')) issues.push('blocker.noProtagonist')
  for (const c of cards) {
    if (!c.role) issues.push(`cast.missingRole:${c.stem}`)
    if (!c.status) issues.push(`cast.missingStatus:${c.stem}`)
  }
  for (const e of buildRelationEdges(cards)) {
    if (e.unknownTarget) issues.push(`cast.unknownStem:${e.from}→${e.to}`)
    else if (e.missingBackEdge) issues.push(`cast.missingBackEdge:${e.from}→${e.to}`)
  }
  return issues
}

// ---------------------------------------------------------------------------
// Units
// ---------------------------------------------------------------------------

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
  who: string[]
  pov: string
  where: string
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
  entry: string
  desire: string
  obstacle: string
  choice: string
  payoff: string
  pleasure: string
  forbidden: string
  wordTarget: string
  hookType: string
  hookOut: string
  onStage: string[]
  pov: string
  chapterRange: { from: number; to: number } | null
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

export function compareUnits(a: NovelUnitEntry, b: NovelUnitEntry): number {
  if (a.volume !== b.volume) return a.volume - b.volume
  return a.index - b.index
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
  const body: string[] = []
  for (const line of lines) {
    if (!on) {
      if (line === `${section}:` || line.startsWith(`${section}:`)) on = true
      continue
    }
    // Next top-level key ends the section. Nested list items stay inside.
    if (/^\S/.test(line)) break
    body.push(line)
  }
  let minIndent = Infinity
  for (const line of body) {
    const m = line.match(/^(\s+)-\s+/)
    if (m) minIndent = Math.min(minIndent, m[1].length)
  }
  if (!Number.isFinite(minIndent)) return []
  const blocks: string[][] = []
  let cur: string[] | null = null
  for (const line of body) {
    const m = line.match(/^(\s+)-\s+/)
    if (m && m[1].length === minIndent) {
      if (cur && cur.length) blocks.push(cur)
      cur = [line]
      continue
    }
    if (cur) cur.push(line)
  }
  if (cur && cur.length) blocks.push(cur)
  return blocks
}

function blockField(block: string[], key: string): string {
  const re = new RegExp(`(?:^|\\s)${key}:\\s*(.*)$`)
  for (const line of block) {
    const m = line.match(re)
    if (m) {
      let v = m[1].trim()
      const hash = v.search(/\s+#/)
      if (hash >= 0 && !/^["'\[]/.test(v)) v = v.slice(0, hash).trim()
      return v.replace(/^["']|["']$/g, '').trim()
    }
  }
  return ''
}

function blockChild(block: string[], parent: string, child: string): string {
  let inParent = false
  let parentIndent = 0
  const parentRe = new RegExp(`^(\\s*)${parent}:\\s*(.*)$`)
  const childRe = new RegExp(`^\\s+${child}:\\s*(.*)$`)
  for (const line of block) {
    const pm = line.match(parentRe)
    if (pm) {
      const inline = pm[2].trim()
      if (inline && !inline.startsWith('|')) return inline.replace(/^["']|["']$/g, '')
      inParent = true
      parentIndent = pm[1].length
      continue
    }
    if (!inParent) continue
    if (line.trim()) {
      const indent = line.match(/^(\s*)/)?.[1].length ?? 0
      if (indent <= parentIndent) break
    }
    const cm = line.match(childRe)
    if (cm) return cm[1].trim().replace(/^["']|["']$/g, '')
  }
  return ''
}

function blockList(block: string[], key: string): string[] {
  const v = blockField(block, key)
  const m = v.match(/^\[([^\]]*)\]$/)
  if (!m) return v ? [v] : []
  return m[1]
    .split(',')
    .map((s) => s.trim().replace(/^["']|["']$/g, ''))
    .filter(Boolean)
}

export function parseUnitOutlineYaml(raw: string): NovelUnitOutlineFields {
  const scenes: NovelUnitScene[] = sectionBlocks(raw, 'scenes').map((block) => ({
    id: blockField(block, 'id'),
    beat: blockField(block, 'beat') || blockField(block, 'name'),
    chapter: Number.parseInt(blockField(block, 'chapter'), 10) || 0,
    who: blockList(block, 'who'),
    pov: blockField(block, 'pov'),
    where: blockField(block, 'where') || blockField(block, '章位'),
  })).filter((s) => s.id || s.beat)
  const chapters: NovelUnitChapterCut[] = sectionBlocks(raw, 'chapters').map((block) => {
    const hookScalar = blockField(block, 'cut_hook')
    const hookText = hookScalar || blockChild(block, 'cut_hook', 'text')
    const hookType = hookScalar ? '' : blockChild(block, 'cut_hook', 'type')
    return {
      chapter: Number.parseInt(blockField(block, 'chapter'), 10) || 0,
      title: blockField(block, 'title_working') || blockField(block, 'title'),
      cutHook: [hookType, hookText].filter(Boolean).join(' · '),
      wordShare: blockField(block, 'word_share'),
    }
  })
  const range = raw.match(/^chapter_range:\s*\[\s*(\d+)\s*,\s*(\d+)\s*\]/m)
  return {
    unitId: yamlScalar(raw, 'unit_id'),
    status: yamlScalar(raw, 'status').toLowerCase() || 'proposed',
    title: yamlScalar(raw, 'title_working') || yamlScalar(raw, 'title'),
    functionText: yamlScalar(raw, 'function'),
    entry: yamlScalar(raw, 'entry') || yamlScalar(raw, 'causal_entry'),
    desire: yamlScalar(raw, 'desire') || yamlScalar(raw, 'protagonist_goal'),
    obstacle: yamlScalar(raw, 'obstacle') || yamlScalar(raw, 'core_obstacle'),
    choice: yamlScalar(raw, 'choice') || yamlScalar(raw, 'key_choice'),
    payoff: yamlScalar(raw, 'payoff'),
    pleasure: yamlScalar(raw, 'pleasure'),
    forbidden: yamlScalar(raw, 'forbidden'),
    wordTarget: yamlScalar(raw, 'word_target'),
    hookType: yamlNestedScalar(raw, 'next_hook', 'type'),
    hookOut: yamlNestedScalar(raw, 'next_hook', 'out'),
    onStage: yamlList(raw, 'on_stage'),
    pov: yamlScalar(raw, 'pov'),
    chapterRange: range ? { from: Number(range[1]), to: Number(range[2]) } : null,
    scenes,
    chapters,
  }
}

export function applyUnitOutline(entry: NovelUnitEntry, raw: string): NovelUnitEntry {
  const parsed = parseUnitOutlineYaml(raw)
  const from = parsed.chapters[0]?.chapter ?? 0
  const to = parsed.chapters.length ? parsed.chapters[parsed.chapters.length - 1].chapter : 0
  return {
    ...entry,
    chapterFrom: parsed.chapterRange ? parsed.chapterRange.from : from,
    chapterTo: parsed.chapterRange ? parsed.chapterRange.to : to,
    label: parsed.title ? `${entry.unitId} · ${parsed.title}` : entry.unitId,
  }
}

/**
 * proposed + no scenes → pending_outline; accepted (no prose) → ready;
 * prose → drafted / review_fail; reviewed + range ≤ last_committed_ch → finalized.
 */
export function inferUnitPhase(
  entry: NovelUnitEntry,
  lastCommittedCh: number,
  outlineRaw?: string,
  reviewRaw?: string,
): NovelUnitPhase {
  const outline = outlineRaw ? parseUnitOutlineYaml(outlineRaw) : null
  const status = outline?.status ?? ''
  const verdict = reviewRaw ? parseReviewVerdict(reviewRaw) : null
  const rangeTo = entry.chapterTo || outline?.chapterRange?.to || 0
  if (entry.prose) {
    if (status === 'reviewed' && rangeTo > 0 && rangeTo <= lastCommittedCh) return 'finalized'
    if (verdict === 'FAIL') return 'review_fail'
    return 'drafted'
  }
  if (entry.outline) {
    // Only an accepted outline is ready to draft. A proposed head stays
    // pending even if some scene rows were filled in — status is the contract.
    if (status === 'accepted' || status === 'drafted' || status === 'reviewed') return 'ready'
    return 'pending_outline'
  }
  return 'pending_outline'
}

export function isUnitPhasePending(phase: NovelUnitPhase): boolean {
  return phase !== 'finalized'
}

export function buildUnitPhases(
  entries: NovelUnitEntry[],
  lastCommittedCh: number,
  outlineRaws: Record<string, string> = {},
  reviewRaws: Record<string, string> = {},
): Record<string, NovelUnitPhase> {
  const out: Record<string, NovelUnitPhase> = {}
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

// ---------------------------------------------------------------------------
// Book context → phase → primary action
// ---------------------------------------------------------------------------

export interface NovelVolumeInfo {
  volume: number
  /** File name under outline/volumes (or outline root for legacy names). */
  fileName: string
  /** Stems under 「本卷人物」. */
  cast: string[]
  rows: VolumeUnitRow[]
}

export interface NovelBookContext {
  bookId: string
  state: NovelExtendedState
  entries: NovelUnitEntry[]
  unitPhases: Record<string, NovelUnitPhase>
  /** Parsed unit YAML by unitId (only for entries with an outline file). */
  unitOutlines: Record<string, NovelUnitOutlineFields>
  volumes: NovelVolumeInfo[]
  cast: NovelCastCard[]
  hasBookOutline: boolean
  /** Old layout detected (ledger.md without facts.md, chapters/ prose, `## chNNN` in facts). */
  legacy: boolean
}

export interface NovelPrimaryDecision {
  action: NovelStageAction
  phase: NovelPipelinePhase
  unitId?: string
  volume?: number
  /** For outline-batch: proposed unit ids in this round (≤4). */
  batchUnits?: string[]
  /** i18n blocker keys (`blocker.*`) or raw state blockers. */
  blockers: string[]
  allowed: boolean
}

export interface NovelBookPipeline {
  phase: NovelPipelinePhase
  /** Volume the phase is talking about (latest volume file, or 1). */
  volume: number
  gates: { knowledge: GateStatus; asset: GateStatus; qc: GateStatus }
  blockers: string[]
  progress: { finalized: number; outlined: number; total: number; percent: number }
  primary: NovelPrimaryDecision | null
}

/** Latest volume by file; 0 when no volume outline exists. */
export function currentVolumeNumber(ctx: Pick<NovelBookContext, 'volumes'>): number {
  let max = 0
  for (const v of ctx.volumes) if (v.volume > max) max = v.volume
  return max
}

export function volumeInfo(ctx: Pick<NovelBookContext, 'volumes'>, volume: number): NovelVolumeInfo | null {
  return ctx.volumes.find((v) => v.volume === volume) ?? null
}

function unitsOfVolume(ctx: NovelBookContext, volume: number): NovelUnitEntry[] {
  return ctx.entries.filter((e) => e.volume === volume).sort(compareUnits)
}

function proposedUnits(ctx: NovelBookContext, volume: number): NovelUnitEntry[] {
  return unitsOfVolume(ctx, volume).filter((e) => (ctx.unitPhases[e.unitId] ?? 'pending_outline') === 'pending_outline')
}

/** Asset gate for writing: a canon protagonist exists and no `candidate` stands on stage. */
export function assetGateStatus(ctx: NovelBookContext): GateStatus {
  if (!ctx.cast.length) return ctx.state.gates.asset === 'fail' ? 'fail' : 'unknown'
  const protagonist = ctx.cast.some((c) => c.role === 'protagonist' && c.status === 'canon')
  return protagonist ? 'pass' : 'fail'
}

export function inferBookPipelinePhase(ctx: NovelBookContext): NovelPipelinePhase {
  if (ctx.state.stage === 'idle') return 'idle'
  const vol = currentVolumeNumber(ctx)
  if (vol === 0) return 'planning'
  const units = unitsOfVolume(ctx, vol)
  if (!units.length) return 'planning'
  if (proposedUnits(ctx, vol).length) return 'outlining'
  if (units.some((e) => ctx.unitPhases[e.unitId] !== 'finalized')) return 'units'
  return 'planning'
}

function writeBlockers(ctx: NovelBookContext, unitId: string): string[] {
  const blockers: string[] = []
  const outline = ctx.unitOutlines[unitId]
  const byStem = new Map(ctx.cast.map((c) => [c.stem, c]))
  if (assetGateStatus(ctx) !== 'pass') blockers.push(ctx.cast.length ? 'blocker.noProtagonist' : 'blocker.noCast')
  if (outline) {
    const vol = volumeInfo(ctx, unitMetaFromName(unitId)?.volume ?? 0)
    for (const stem of outline.onStage) {
      const card = byStem.get(stem)
      if (!card) blockers.push(`blocker.unknownStem:${stem}`)
      else if (card.status !== 'canon') blockers.push(`blocker.candidateOnStage:${stem}`)
      if (vol && vol.cast.length && !vol.cast.includes(stem)) blockers.push(`blocker.castOutsideVolume:${stem}`)
    }
  }
  return blockers
}

function decision(
  action: NovelStageAction,
  phase: NovelPipelinePhase,
  extra: Partial<Omit<NovelPrimaryDecision, 'action' | 'phase' | 'blockers' | 'allowed'>>,
  blockers: string[],
): NovelPrimaryDecision {
  return { action, phase, ...extra, blockers, allowed: blockers.length === 0 }
}

/**
 * The one function that decides the primary button.
 * planning → plan (volume N); outlining → outline-batch (first ≤4 proposed);
 * units → write / finalize for the selected unit (or the first unfinished one);
 * a finalized selection jumps to the next unfinished unit, or next-volume.
 */
export function selectPrimaryAction(ctx: NovelBookContext, selectedUnit?: string | null): NovelPrimaryDecision {
  const stateBlockers = [...ctx.state.blockers]
  if (ctx.legacy) return decision('migrate', 'planning', {}, [])

  const phase = inferBookPipelinePhase(ctx)
  const vol = currentVolumeNumber(ctx)

  if (phase === 'idle') {
    return decision('next-volume', 'idle', { volume: vol + 1 }, stateBlockers)
  }

  if (phase === 'planning') {
    if (vol > 0 && unitsOfVolume(ctx, vol).every((e) => ctx.unitPhases[e.unitId] === 'finalized') && unitsOfVolume(ctx, vol).length) {
      return decision('next-volume', phase, { volume: vol + 1 }, stateBlockers)
    }
    return decision('plan', phase, { volume: vol || 1 }, stateBlockers)
  }

  if (phase === 'outlining') {
    const batch = proposedUnits(ctx, vol).slice(0, 4).map((e) => e.unitId)
    return decision('outline-batch', phase, { volume: vol, batchUnits: batch, unitId: batch[0] }, stateBlockers)
  }

  // units
  const unitAction = (entry: NovelUnitEntry): NovelPrimaryDecision | null => {
    const ph = ctx.unitPhases[entry.unitId] ?? 'pending_outline'
    if (ph === 'pending_outline') {
      const batch = proposedUnits(ctx, entry.volume).slice(0, 4).map((e) => e.unitId)
      return decision('outline-batch', phase, { volume: entry.volume, batchUnits: batch.length ? batch : [entry.unitId], unitId: entry.unitId }, stateBlockers)
    }
    if (ph === 'ready') {
      return decision('write', phase, { unitId: entry.unitId, volume: entry.volume }, [...stateBlockers, ...writeBlockers(ctx, entry.unitId)])
    }
    if (ph === 'drafted' || ph === 'review_fail') {
      return decision('finalize', phase, { unitId: entry.unitId, volume: entry.volume }, stateBlockers)
    }
    return null
  }

  const selected = selectedUnit ? ctx.entries.find((e) => e.unitId === selectedUnit) : undefined
  if (selected) {
    const d = unitAction(selected)
    if (d) return d
    // finalized selection → next unfinished unit in the same volume
    for (const e of unitsOfVolume(ctx, selected.volume)) {
      if (e.index <= selected.index) continue
      const dd = unitAction(e)
      if (dd) return dd
    }
    if (selected.volume !== vol) {
      return decision('next-volume', phase, { volume: selected.volume + 1 }, stateBlockers)
    }
  }

  for (const e of unitsOfVolume(ctx, vol)) {
    const d = unitAction(e)
    if (d) return d
  }
  return decision('next-volume', phase, { volume: vol + 1 }, stateBlockers)
}

export function computeBookPipeline(ctx: NovelBookContext, selectedUnit?: string | null): NovelBookPipeline {
  const phase = inferBookPipelinePhase(ctx)
  const vol = currentVolumeNumber(ctx) || 1
  const total = ctx.entries.length
  const finalized = ctx.entries.filter((e) => ctx.unitPhases[e.unitId] === 'finalized').length
  const outlined = ctx.entries.filter((e) => ctx.unitPhases[e.unitId] !== 'pending_outline').length
  const percent = total > 0 ? Math.min(100, Math.round((finalized / total) * 100)) : 0

  const primary = selectPrimaryAction(ctx, selectedUnit)
  // Planning may be all-candidate; the canon-protagonist check applies once units are being written.
  const asset =
    phase === 'outlining' || phase === 'units' ? assetGateStatus(ctx) : ctx.state.gates.asset
  const gates = {
    knowledge: ctx.state.gates.knowledge,
    asset,
    qc: ctx.state.gates.qc,
  }
  return {
    phase,
    volume: vol,
    gates,
    blockers: [...ctx.state.blockers],
    progress: { finalized, outlined, total, percent },
    primary,
  }
}

/** Secondary (「更多」) actions available for a selected unit / cast card. */
export function canRunAction(
  action: NovelStageAction,
  ctx: NovelBookContext,
  unitId?: string,
): { allowed: boolean; blockers: string[] } {
  const blockers: string[] = []
  const phase = unitId ? ctx.unitPhases[unitId] : undefined
  switch (action) {
    case 'write':
      if (!unitId) blockers.push('blocker.noUnit')
      else if (phase !== 'ready') blockers.push(phase === 'pending_outline' ? 'blocker.needOutline' : phase === 'finalized' ? 'blocker.alreadyFinalized' : 'blocker.alreadyDrafted')
      if (unitId) blockers.push(...writeBlockers(ctx, unitId))
      break
    case 'finalize':
    case 'expand':
    case 'review':
    case 'polish':
      if (!unitId) blockers.push('blocker.noUnit')
      else if (phase === 'pending_outline' || phase === 'ready') blockers.push('blocker.needDraft')
      else if (phase === 'finalized') blockers.push('blocker.alreadyFinalized')
      break
    case 'contract-one':
      if (!unitId) blockers.push('blocker.noUnit')
      else if (phase !== 'pending_outline') blockers.push('blocker.alreadyOutlined')
      break
    case 'outline-batch':
      if (!ctx.entries.some((e) => ctx.unitPhases[e.unitId] === 'pending_outline')) blockers.push('blocker.noProposed')
      break
    default:
      break
  }
  if (action !== 'plan' && action !== 'init' && action !== 'migrate' && action !== 'cast-fix' && action !== 'next-volume') {
    blockers.push(...ctx.state.blockers)
  }
  return { allowed: blockers.length === 0, blockers }
}

// ---------------------------------------------------------------------------
// Composer prefill
// ---------------------------------------------------------------------------

export function novelActionSkillId(action: NovelStageAction): NovelSkillId {
  switch (action) {
    case 'init':
    case 'migrate':
      return 'novel-setup'
    case 'plan':
    case 'next-volume':
    case 'cast-fix':
      return 'novel-plan'
    case 'finalize':
    case 'expand':
    case 'review':
    case 'polish':
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
  if (action === 'write') {
    return `技能 ${skillId} · 意图 ${action} — 按该技能 Intent→Load 表 read_skill；写正文先跑 gate preflight --unit，只消费 ### CONTEXT；落盘一份单元正文后停下（定稿另轮）。`
  }
  if (action === 'outline-batch' || action === 'contract-one') {
    return `技能 ${skillId} · 意图 ${action} — 按该技能 Intent→Load 表 read_skill；填完跑 gate lint-units --volume；本轮不写正文。`
  }
  if (skillId === 'novel-review') {
    return `技能 ${skillId} · 意图 ${action} — 按该技能 Intent→Load 表 read_skill；定稿轮（qc-pack → 审 → 一次 Commit → postcommit），与写作分 turn。`
  }
  if (action === 'plan' || action === 'next-volume') {
    return `技能 ${skillId} · 意图 ${action} — 按该技能 Intent→Load 表 read_skill；卷纲批准后跑 gate accept-volume --volume。`
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
    `阶段 ${pipeline.phase} · 卷 ${volumeId(pipeline.volume)} · 动作 ${action} · 技能 ${skillId}`,
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

/** Intent + project paths only; no knowledge prose, no reference paths. */
export function buildNovelStagePrefill(action: NovelStageAction, ctx: NovelStagePrefillCtx): string {
  const bookId = (ctx.bookId ?? '').trim() || '<book-id>'
  const root = `novel/${bookId}`
  const vol = ctx.volume && ctx.volume > 0 ? ctx.volume : 0
  const volTag = vol > 0 ? volumeId(vol) : 'vNN'
  const unitId = (ctx.unitId ?? '').trim() || 'vNN-U#'
  const unitPath = ctx.unitPath || `${root}/units/${unitId}.md`
  const outlinePath = `${root}/outline/units/${unitId}.yaml`
  const volumePath = `${root}/outline/volumes/${volTag}.md`
  const batch = (ctx.batchUnits ?? []).filter(Boolean)
  const batchText = batch.length ? batch.join('、') : `${volTag} 前 ≤4 个 status: proposed 单元`

  switch (action) {
    case 'init':
      return [
        '开一本新书并立项。',
        '用一次 ask_user 收齐：题材（八题材之一）、读者承诺、篇幅/平台、POV、禁忌。',
        `exec_shell gate --action init --book-id <slug> --title <书名> --genre <题材> 建树到 ${root.replace(bookId, '<book-id>')}/，再填 bible / state / world。人物卡留给规划轮。`,
      ].join('\n')
    case 'migrate':
      return [
        `旧书迁移（${root}/）。`,
        'exec_shell gate --action doctor 看 [migrate] 项，再 --action migrate --book-id；核对 continuity/commits/migrate-<date>.md，确认 genre 后清 blocker。',
      ].join('\n')
    case 'plan':
      if (ctx.volumeOutlineExists) {
        return [
          `第 ${vol || 'N'} 卷卷纲已在 ${volumePath}，等我批准。`,
          '批准后 exec_shell gate --action accept-volume --volume ' + volTag + '：本卷人物 candidate → canon，按单元索引种出 proposed 细纲头。不写细纲正文。',
        ].join('\n')
      }
      return [
        `规划一轮（书：${root}/）：人物卡（canon/cast/<stem>.md，status: candidate，按 role 填最小必填）+ 总纲 outline/book_outline.md + 第 ${vol || 1} 卷卷纲 ${volumePath}（单元索引 + 本卷人物 stem）。`,
        '止于卷纲；不写细纲、不写正文。写完停下等我批准，批准后再跑 gate accept-volume。',
      ].join('\n')
    case 'next-volume':
      return [
        `上一卷已全部定稿。为第 ${vol || 'N+1'} 卷写卷纲 → ${volumePath}（单元索引 + 本卷人物；新角色先 candidate 卡）。`,
        '卷末先在 continuity/summaries/ 上一卷文件末尾写卷总结。写完停下等我批准，批准后跑 gate accept-volume --volume ' + volTag + '。',
      ].join('\n')
    case 'outline-batch':
      return [
        `一批细纲（卷 ${volTag}）：把 ${batchText} 从 proposed 填成 accepted → ${root}/outline/units/。`,
        '每个补 on_stage（⊆ 卷纲本卷人物）/ pov / 合同 / scenes / chapters / state_deltas；不改 function 与 next_hook.type。',
        `填完 exec_shell gate --action lint-units --volume ${volTag}，FAIL 只补失败单元。本轮不写正文。`,
      ].join('\n')
    case 'contract-one':
      return [
        `只填单元 ${unitId} 的细纲 → ${outlinePath}（proposed → accepted）。`,
        `补 on_stage / pov / 合同 / 场面 / 章切口；gate lint-units --volume ${unitId.split('-')[0]} 通过后停下。`,
      ].join('\n')
    case 'write':
      return [
        `写单元 ${unitId} 正文，一份文件 ${unitPath}。`,
        '先 exec_shell gate --action preflight --unit ' + unitId + '，只消费 ### CONTEXT（题材文 + 单元卡 + on_stage 人物）；不再读细纲 / 人物卡 / facts。',
        '章与章用单独一行 --- 分隔，标题为 ## 第N章。落盘后停下；定稿另开一轮（可换模）。一轮只写这一个单元。',
      ].join('\n')
    case 'finalize':
      return [
        `定稿单元 ${unitId}（${unitPath}）。`,
        `先 exec_shell gate --action qc-pack --unit ${unitId}：expand_needed=yes 才扩写，HITS 非空才润色；再 10 维审；PASS 不写 review 文件。`,
        `一次补丁 Commit（continuity/summaries/${unitId.split('-')[0]}.md 章摘要 + facts 游标 + 人物卡关系两列 + 细纲 reviewed + state）→ gate --action postcommit --unit ${unitId} exit 0。不拆多次 Commit。`,
      ].join('\n')
    case 'expand':
      return [
        `只扩写 ${unitPath}（扩场面，不按章注水；≤3 种技术）。`,
        `改完复跑 gate qc-pack --unit ${unitId}，expand_needed 应为 no。保持章标题和 ---。不 Commit。`,
      ].join('\n')
    case 'review':
      return [
        `只审单元 ${unitId}（${unitPath}）：gate qc-pack --unit 后做 10 维审。`,
        `PASS 只更新 gates.qc；FAIL / 深审写 reviews/${unitId}-review.md。不 Commit。`,
      ].join('\n')
    case 'polish':
      return [
        `只给 ${unitPath} 去 AI 味：按 gate qc-pack / scan-deslop --unit ${unitId} 的 ### HITS 行号定点改。`,
        '不改情节 Canon；复扫 exit 0 后停下。不 Commit。',
      ].join('\n')
    case 'cast-fix': {
      const stem = (ctx.stem ?? '').trim() || '<stem>'
      return [
        `补人物卡 ${root}/canon/cast/${stem}.md：按 role 的完整度表补缺项（三锚点 / 语言习惯 / 台词样例 / 关系表对边）。`,
        '不改 status（提升只由 accept-volume 做）。补完 exec_shell gate --action cast-lint --book-id ' + bookId + '。',
      ].join('\n')
    }
    default:
      return ''
  }
}

/** Legacy layout heuristics — the gate's `doctor` is authoritative; this only flips the CTA to 「迁移」. */
export function detectLegacyLayout(input: {
  continuityFiles: NovelFileNode[]
  legacyChapterFiles: NovelFileNode[]
  factsRaw?: string
}): boolean {
  const names = new Set(input.continuityFiles.map((f) => f.name.toLowerCase()))
  if (names.has('ledger.md') && !names.has('facts.md')) return true
  if (input.legacyChapterFiles.some((f) => !f.isDir && /\.md$/i.test(f.name))) return true
  if (input.factsRaw && /^##\s+ch\d{3}/m.test(input.factsRaw)) return true
  return false
}
