import { ref, type Ref } from 'vue'
import { asArray, fetchJSON } from '@/api/client'
import {
  applyUnitOutline,
  buildUnitEntries,
  buildUnitPhases,
  isBookOutlineName,
  mergeVolumeOutlineFiles,
  novelActiveBookPath,
  novelBookDir,
  novelCanonDir,
  novelCastDir,
  novelContinuityDir,
  novelOutlineDir,
  novelReviewsDir,
  novelStatePath,
  novelUnitOutlinesDir,
  novelUnitReviewPath,
  novelUnitsDir,
  novelVolumesDir,
  parseBookOutlineVolumeRows,
  parseNovelStateExtended,
  parseVolumeUnitRows,
  sortWorkbenchDocNodes,
  type BookOutlineVolumeRow,
  type NovelExtendedState,
  type NovelFileNode,
  type NovelStateSummary,
  type NovelUnitEntry,
  type VolumeUnitRow,
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
  /** Committed units / units that have an outline or prose file. */
  progress: { committed: number; total: number } | null
}

export function useNovelBookLoader(projectId: Ref<string | null | undefined>) {
  const loading = ref(false)
  const books = ref<ShelfBookRow[]>([])
  const selectedBookId = ref<string | null>(null)
  const activeBookId = ref<string | null>(null)
  const unitEntries = ref<NovelUnitEntry[]>([])
  const continuityFiles = ref<NovelFileNode[]>([])
  const outlineFiles = ref<NovelFileNode[]>([])
  const volumeFiles = ref<NovelFileNode[]>([])
  const reviewFiles = ref<NovelFileNode[]>([])
  const canonFiles = ref<NovelFileNode[]>([])
  const castFiles = ref<NovelFileNode[]>([])
  const extendedState = ref<NovelExtendedState | null>(null)
  const outlineRaws = ref<Record<string, string>>({})
  const reviewRaws = ref<Record<string, string>>({})
  const proseRaws = ref<Record<string, string>>({})
  const bookState = ref<NovelStateSummary | null>(null)
  const bookOutlineRows = ref<BookOutlineVolumeRow[]>([])
  const volumeUnitRows = ref<Record<string, VolumeUnitRow[]>>({})

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
          try {
            outlines[e.unitId] = await readFile(unitNodePath(bookId, e.outline, 'outline'))
          } catch {
            /* ignore */
          }
        }
        if (e.prose) {
          try {
            proses[e.unitId] = await readFile(unitNodePath(bookId, e.prose, 'prose'))
          } catch {
            /* ignore */
          }
          try {
            reviews[e.unitId] = await readFile(novelUnitReviewPath(bookId, e.unitId))
          } catch {
            /* ignore */
          }
        }
      }),
    )
    outlineRaws.value = outlines
    reviewRaws.value = reviews
    proseRaws.value = proses
    unitEntries.value = entries.map((e) =>
      outlines[e.unitId] ? applyUnitOutline(e, outlines[e.unitId]) : e,
    )
  }

  async function loadOutlinePreviews(bookId: string) {
    bookOutlineRows.value = []
    volumeUnitRows.value = {}
    const bookFile = outlineFiles.value.find((f) => isBookOutlineName(f.name))
    if (bookFile) {
      try {
        const raw = await readFile(nodePath(bookId, novelOutlineDir(bookId), bookFile))
        bookOutlineRows.value = parseBookOutlineVolumeRows(raw)
      } catch {
        /* ignore */
      }
    }
    const previews: Record<string, VolumeUnitRow[]> = {}
    await Promise.all(
      mergeVolumeOutlineFiles(outlineFiles.value, volumeFiles.value).map(async (f) => {
        try {
          previews[f.name] = parseVolumeUnitRows(
            await readFile(volumeNodePath(bookId, f, volumeFiles.value)),
          )
        } catch {
          previews[f.name] = []
        }
      }),
    )
    volumeUnitRows.value = previews
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
  ): Promise<{ committed: number; total: number } | null> {
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
          try {
            raws[e.unitId] = await readFile(unitNodePath(bookId, e.outline, 'outline'))
          } catch {
            /* ignore */
          }
        }),
      )
      const applied = entries.map((e) => (raws[e.unitId] ? applyUnitOutline(e, raws[e.unitId]) : e))
      const phases = buildUnitPhases(applied, state.lastCommittedCh, raws, {})
      const total = applied.filter((e) => e.outline || e.prose).length
      const committed = applied.filter((e) => phases[e.unitId] === 'committed').length
      return { committed, total }
    } catch {
      return { committed: 0, total: 0 }
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

  async function openBook(bookId: string) {
    selectedBookId.value = bookId
    loading.value = true
    try {
      bookState.value = await loadState(bookId)
      const [outlineUnitNodes, proseNodes, contNodes, outNodes, volNodes, revNodes, canonNodes, castNodes] =
        await Promise.all([
          listDirSoft(novelUnitOutlinesDir(bookId)),
          listDirSoft(novelUnitsDir(bookId)),
          listDirSoft(novelContinuityDir(bookId)),
          listDirSoft(novelOutlineDir(bookId)),
          listDirSoft(novelVolumesDir(bookId)),
          listDirSoft(novelReviewsDir(bookId)),
          listDirSoft(novelCanonDir(bookId)),
          listDirSoft(novelCastDir(bookId)),
        ])
      continuityFiles.value = sortWorkbenchDocNodes(contNodes)
      outlineFiles.value = sortWorkbenchDocNodes(outNodes)
      volumeFiles.value = sortWorkbenchDocNodes(volNodes)
      reviewFiles.value = sortWorkbenchDocNodes(revNodes)
      canonFiles.value = sortWorkbenchDocNodes(canonNodes)
      castFiles.value = sortWorkbenchDocNodes(castNodes)
      await loadUnitMeta(bookId, buildUnitEntries(outlineUnitNodes, proseNodes))
      await loadOutlinePreviews(bookId)
      void persistActiveBook(bookId)
    } catch {
      unitEntries.value = []
      continuityFiles.value = []
      outlineFiles.value = []
      volumeFiles.value = []
      reviewFiles.value = []
      canonFiles.value = []
      castFiles.value = []
      extendedState.value = null
      outlineRaws.value = {}
      reviewRaws.value = {}
      proseRaws.value = {}
      bookState.value = null
      bookOutlineRows.value = []
      volumeUnitRows.value = {}
    } finally {
      loading.value = false
    }
  }

  function clearBook() {
    selectedBookId.value = null
    unitEntries.value = []
    continuityFiles.value = []
    outlineFiles.value = []
    volumeFiles.value = []
    reviewFiles.value = []
    canonFiles.value = []
    castFiles.value = []
    extendedState.value = null
    outlineRaws.value = {}
    reviewRaws.value = {}
    proseRaws.value = {}
    bookState.value = null
    bookOutlineRows.value = []
    volumeUnitRows.value = {}
  }

  return {
    loading,
    books,
    selectedBookId,
    activeBookId,
    unitEntries,
    continuityFiles,
    outlineFiles,
    volumeFiles,
    reviewFiles,
    canonFiles,
    castFiles,
    extendedState,
    outlineRaws,
    reviewRaws,
    proseRaws,
    bookState,
    bookOutlineRows,
    volumeUnitRows,
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
