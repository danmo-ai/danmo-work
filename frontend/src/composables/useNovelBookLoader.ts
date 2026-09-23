import { ref, type Ref } from 'vue'
import { asArray, fetchJSON } from '@/api/client'
import {
  applyUnitOutline,
  buildUnitEntries,
  buildUnitPhases,
  castStemFromName,
  detectLegacyLayout,
  isBookOutlineName,
  mergeVolumeOutlineFiles,
  novelActiveBookPath,
  novelBookDir,
  novelCanonDir,
  novelCastDir,
  novelContinuityDir,
  novelLegacyChaptersDir,
  novelOutlineDir,
  novelReviewsDir,
  novelStatePath,
  novelSummariesDir,
  novelUnitOutlinesDir,
  novelUnitReviewPath,
  novelUnitsDir,
  novelVolumesDir,
  parseBookOutlineVolumeRows,
  parseCastCard,
  parseNovelStateExtended,
  parseUnitOutlineYaml,
  parseVolumeCast,
  parseVolumeUnitRows,
  sortWorkbenchDocNodes,
  volumeNumFromName,
  type BookOutlineVolumeRow,
  type NovelCastCard,
  type NovelExtendedState,
  type NovelFileNode,
  type NovelStateSummary,
  type NovelUnitEntry,
  type NovelUnitOutlineFields,
  type NovelVolumeInfo,
} from '@/types/novel-workbench'

function nodePath(bookId: string, dir: string, node: NovelFileNode): string {
  const p = (node.path || '').replace(/\\/g, '/')
  if (p) return p
  return `${dir}/${node.name}`
}

function unitNodePath(bookId: string, node: NovelFileNode, kind: 'outline' | 'prose'): string {
  const p = (node.path || '').replace(/\\/g, '/')
  if (p) return p
  const dir = kind === 'outline' ? novelUnitOutlinesDir(bookId) : novelUnitsDir(bookId)
  return `${dir}/${node.name}`
}

function volumeNodePath(
  bookId: string,
  node: NovelFileNode,
  volumeFiles: NovelFileNode[],
): string {
  const p = (node.path || '').replace(/\\/g, '/')
  if (p) return p
  if (volumeFiles.some((v) => v.name === node.name)) {
    return `${novelVolumesDir(bookId)}/${node.name}`
  }
  return `${novelOutlineDir(bookId)}/${node.name}`
}

export type ShelfBookRow = {
  id: string
  path: string
  state: NovelStateSummary | null
  /** Finalized units / units on disk. */
  progress: { finalized: number; total: number } | null
}

export function useNovelBookLoader(projectId: Ref<string | null | undefined>) {
  const loading = ref(false)
  const books = ref<ShelfBookRow[]>([])
  const selectedBookId = ref<string | null>(null)
  const activeBookId = ref<string | null>(null)
  const unitEntries = ref<NovelUnitEntry[]>([])
  const continuityFiles = ref<NovelFileNode[]>([])
  const summaryFiles = ref<NovelFileNode[]>([])
  const outlineFiles = ref<NovelFileNode[]>([])
  const volumeFiles = ref<NovelFileNode[]>([])
  const reviewFiles = ref<NovelFileNode[]>([])
  const canonFiles = ref<NovelFileNode[]>([])
  const castFiles = ref<NovelFileNode[]>([])
  const castCards = ref<NovelCastCard[]>([])
  const castRaws = ref<Record<string, string>>({})
  const extendedState = ref<NovelExtendedState | null>(null)
  const outlineRaws = ref<Record<string, string>>({})
  const unitOutlines = ref<Record<string, NovelUnitOutlineFields>>({})
  const reviewRaws = ref<Record<string, string>>({})
  const proseRaws = ref<Record<string, string>>({})
  const bookState = ref<NovelStateSummary | null>(null)
  const bookOutlineRows = ref<BookOutlineVolumeRow[]>([])
  const volumes = ref<NovelVolumeInfo[]>([])
  const volumeRaws = ref<Record<string, string>>({})
  const legacyLayout = ref(false)

  async function listDir(path: string): Promise<NovelFileNode[]> {
    if (!projectId.value) return []
    const q = path ? `?path=${encodeURIComponent(path)}` : ''
    return asArray(
      await fetchJSON<NovelFileNode[]>(`/projects/${projectId.value}/files${q}`),
    )
  }

  async function listDirSoft(path: string): Promise<NovelFileNode[]> {
    try {
      return await listDir(path)
    } catch {
      return []
    }
  }

  async function readFile(path: string): Promise<string> {
    if (!projectId.value) return ''
    const fc = await fetchJSON<{ content: string }>(
      `/projects/${projectId.value}/files/content?path=${encodeURIComponent(path)}`,
    )
    return fc.content ?? ''
  }

  async function readFileSoft(path: string): Promise<string> {
    try {
      return await readFile(path)
    } catch {
      return ''
    }
  }

  async function loadState(bookId: string): Promise<NovelStateSummary | null> {
    try {
      const raw = await readFile(novelStatePath(bookId))
      extendedState.value = parseNovelStateExtended(raw)
      bookState.value = extendedState.value
      return extendedState.value
    } catch {
      extendedState.value = null
      return null
    }
  }

  async function loadUnitMeta(bookId: string, entries: NovelUnitEntry[]) {
    const outlines: Record<string, string> = {}
    const reviews: Record<string, string> = {}
    const proses: Record<string, string> = {}
    await Promise.all(
      entries.map(async (e) => {
        if (e.outline) {
          const raw = await readFileSoft(unitNodePath(bookId, e.outline, 'outline'))
          if (raw) outlines[e.unitId] = raw
        }
        if (e.prose) {
          const raw = await readFileSoft(unitNodePath(bookId, e.prose, 'prose'))
          if (raw) proses[e.unitId] = raw
          const rev = await readFileSoft(novelUnitReviewPath(bookId, e.unitId))
          if (rev) reviews[e.unitId] = rev
        }
      }),
    )
    outlineRaws.value = outlines
    reviewRaws.value = reviews
    proseRaws.value = proses
    const parsed: Record<string, NovelUnitOutlineFields> = {}
    for (const [id, raw] of Object.entries(outlines)) parsed[id] = parseUnitOutlineYaml(raw)
    unitOutlines.value = parsed
    unitEntries.value = entries.map((e) =>
      outlines[e.unitId] ? applyUnitOutline(e, outlines[e.unitId]) : e,
    )
  }

  async function loadOutlinePreviews(bookId: string) {
    bookOutlineRows.value = []
    volumes.value = []
    volumeRaws.value = {}
    const bookFile = outlineFiles.value.find((f) => isBookOutlineName(f.name))
    if (bookFile) {
      const raw = await readFileSoft(nodePath(bookId, novelOutlineDir(bookId), bookFile))
      if (raw) bookOutlineRows.value = parseBookOutlineVolumeRows(raw)
    }
    const raws: Record<string, string> = {}
    const infos: NovelVolumeInfo[] = []
    await Promise.all(
      mergeVolumeOutlineFiles(outlineFiles.value, volumeFiles.value).map(async (f) => {
        const volume = volumeNumFromName(f.name)
        if (volume == null) return
        const raw = await readFileSoft(volumeNodePath(bookId, f, volumeFiles.value))
        raws[f.name] = raw
        infos.push({
          volume,
          fileName: f.name,
          cast: parseVolumeCast(raw),
          rows: parseVolumeUnitRows(raw, volume),
        })
      }),
    )
    infos.sort((a, b) => a.volume - b.volume)
    volumeRaws.value = raws
    volumes.value = infos
  }

  async function loadCast(bookId: string) {
    const raws: Record<string, string> = {}
    const cards: NovelCastCard[] = []
    await Promise.all(
      castFiles.value
        .filter((f) => !f.isDir && /\.md$/i.test(f.name))
        .map(async (f) => {
          const stem = castStemFromName(f.name)
          const raw = await readFileSoft(f.path || `${novelCastDir(bookId)}/${f.name}`)
          raws[stem] = raw
          cards.push(parseCastCard(raw, stem))
        }),
    )
    cards.sort((a, b) => a.stem.localeCompare(b.stem, undefined, { numeric: true, sensitivity: 'base' }))
    castRaws.value = raws
    castCards.value = cards
  }

  async function persistActiveBook(bookId: string) {
    if (!projectId.value) return
    try {
      await fetchJSON(`/projects/${projectId.value}/files/content`, {
        method: 'PUT',
        body: JSON.stringify({ path: novelActiveBookPath(), content: `${bookId}\n` }),
      })
      activeBookId.value = bookId
    } catch {
      /* read-mostly; missing write permission should not block */
    }
  }

  async function softShelfProgress(
    bookId: string,
    state: NovelStateSummary | null,
  ): Promise<{ finalized: number; total: number } | null> {
    if (!state) return null
    try {
      const [outlineNodes, proseNodes] = await Promise.all([
        listDirSoft(novelUnitOutlinesDir(bookId)),
        listDirSoft(novelUnitsDir(bookId)),
      ])
      const entries = buildUnitEntries(outlineNodes, proseNodes)
      const raws: Record<string, string> = {}
      await Promise.all(
        entries.map(async (e) => {
          if (!e.outline) return
          const raw = await readFileSoft(unitNodePath(bookId, e.outline, 'outline'))
          if (raw) raws[e.unitId] = raw
        }),
      )
      const applied = entries.map((e) => (raws[e.unitId] ? applyUnitOutline(e, raws[e.unitId]) : e))
      const phases = buildUnitPhases(applied, state.lastCommittedCh, raws, {})
      const total = applied.length
      const finalized = applied.filter((e) => phases[e.unitId] === 'finalized').length
      return { finalized, total }
    } catch {
      return { finalized: 0, total: 0 }
    }
  }

  async function loadShelf() {
    if (!projectId.value) {
      books.value = []
      return
    }
    loading.value = true
    try {
      const root = await listDir('')
      const novelDir = root.find((n) => n.isDir && n.name === 'novel')
      if (!novelDir) {
        books.value = []
        return
      }
      const kids = await listDir('novel')
      try {
        const raw = await readFile(novelActiveBookPath())
        activeBookId.value = raw.trim().split('\n')[0] || null
      } catch {
        activeBookId.value = null
      }
      const dirs = kids.filter((n) => n.isDir && !n.name.startsWith('.'))
      const rows = await Promise.all(
        dirs.map(async (d) => {
          let state: NovelStateSummary | null = null
          try {
            const raw = await readFile(novelStatePath(d.name))
            state = parseNovelStateExtended(raw)
          } catch {
            state = null
          }
          const progress = await softShelfProgress(d.name, state)
          return {
            id: d.name,
            path: d.path || novelBookDir(d.name),
            state,
            progress,
          }
        }),
      )
      rows.sort((a, b) => a.id.localeCompare(b.id, undefined, { sensitivity: 'base' }))
      books.value = rows
    } catch {
      books.value = []
    } finally {
      loading.value = false
    }
  }

  function resetBook() {
    unitEntries.value = []
    continuityFiles.value = []
    summaryFiles.value = []
    outlineFiles.value = []
    volumeFiles.value = []
    reviewFiles.value = []
    canonFiles.value = []
    castFiles.value = []
    castCards.value = []
    castRaws.value = {}
    extendedState.value = null
    outlineRaws.value = {}
    unitOutlines.value = {}
    reviewRaws.value = {}
    proseRaws.value = {}
    bookState.value = null
    bookOutlineRows.value = []
    volumes.value = []
    volumeRaws.value = {}
    legacyLayout.value = false
  }

  async function openBook(bookId: string) {
    selectedBookId.value = bookId
    loading.value = true
    try {
      bookState.value = await loadState(bookId)
      const [
        outlineUnitNodes,
        proseNodes,
        contNodes,
        sumNodes,
        outNodes,
        volNodes,
        revNodes,
        canonNodes,
        castNodes,
        legacyChapterNodes,
      ] = await Promise.all([
        listDirSoft(novelUnitOutlinesDir(bookId)),
        listDirSoft(novelUnitsDir(bookId)),
        listDirSoft(novelContinuityDir(bookId)),
        listDirSoft(novelSummariesDir(bookId)),
        listDirSoft(novelOutlineDir(bookId)),
        listDirSoft(novelVolumesDir(bookId)),
        listDirSoft(novelReviewsDir(bookId)),
        listDirSoft(novelCanonDir(bookId)),
        listDirSoft(novelCastDir(bookId)),
        listDirSoft(novelLegacyChaptersDir(bookId)),
      ])
      continuityFiles.value = sortWorkbenchDocNodes(contNodes)
      summaryFiles.value = sortWorkbenchDocNodes(sumNodes)
      outlineFiles.value = sortWorkbenchDocNodes(outNodes)
      volumeFiles.value = sortWorkbenchDocNodes(volNodes)
      reviewFiles.value = sortWorkbenchDocNodes(revNodes)
      canonFiles.value = sortWorkbenchDocNodes(canonNodes)
      castFiles.value = sortWorkbenchDocNodes(castNodes)
      await Promise.all([
        loadUnitMeta(bookId, buildUnitEntries(outlineUnitNodes, proseNodes)),
        loadOutlinePreviews(bookId),
        loadCast(bookId),
      ])
      const facts = continuityFiles.value.find((f) => f.name.toLowerCase() === 'facts.md')
      const factsRaw = facts ? await readFileSoft(nodePath(bookId, novelContinuityDir(bookId), facts)) : ''
      legacyLayout.value = detectLegacyLayout({
        continuityFiles: continuityFiles.value,
        legacyChapterFiles: legacyChapterNodes,
        factsRaw,
      })
      void persistActiveBook(bookId)
    } catch {
      resetBook()
    } finally {
      loading.value = false
    }
  }

  function clearBook() {
    selectedBookId.value = null
    resetBook()
  }

  return {
    loading,
    books,
    selectedBookId,
    activeBookId,
    unitEntries,
    continuityFiles,
    summaryFiles,
    outlineFiles,
    volumeFiles,
    reviewFiles,
    canonFiles,
    castFiles,
    castCards,
    castRaws,
    extendedState,
    outlineRaws,
    unitOutlines,
    reviewRaws,
    proseRaws,
    bookState,
    bookOutlineRows,
    volumes,
    volumeRaws,
    legacyLayout,
    listDir,
    listDirSoft,
    readFile,
    loadShelf,
    openBook,
    clearBook,
    nodePath,
    unitNodePath,
    volumeNodePath: (bookId: string, node: NovelFileNode) =>
      volumeNodePath(bookId, node, volumeFiles.value),
  }
}
