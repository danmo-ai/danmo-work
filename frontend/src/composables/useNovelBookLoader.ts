import { ref, type Ref } from 'vue'
import { asArray, fetchJSON } from '@/api/client'
import {
  buildChapterEntries,
  isBookOutlineName,
  isNovelContractName,
  mergeVolumeOutlineFiles,
  novelActiveBookPath,
  novelBatchFreezePath,
  novelBookDir,
  novelCanonDir,
  novelCastDir,
  novelChapterReviewPath,
  novelChaptersDir,
  novelContinuityDir,
  novelOutlineDir,
  novelReviewsDir,
  novelStatePath,
  novelVolumesDir,
  parseBatchFreezeYaml,
  parseBookOutlineVolumeRows,
  parseNovelStateExtended,
  parseVolumeUnitRows,
  sortWorkbenchDocNodes,
  type BookOutlineVolumeRow,
  type NovelChapterEntry,
  type NovelExtendedState,
  type NovelFileNode,
  type NovelStateSummary,
  type VolumeUnitRow,
} from '@/types/novel-workbench'

function nodePath(bookId: string, dir: string, node: NovelFileNode): string {
  const p = (node.path || '').replace(/\\/g, '/')
  if (p) return p
  return `${dir}/${node.name}`
}

function chapterNodePath(bookId: string, node: NovelFileNode): string {
  const p = (node.path || '').replace(/\\/g, '/')
  if (p) return p
  return `${novelChaptersDir(bookId)}/${node.name}`
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
  /** Soft chapter progress when chapter dir was listed. */
  progress: { committed: number; total: number } | null
}

export function useNovelBookLoader(projectId: Ref<string | null | undefined>) {
  const loading = ref(false)
  const books = ref<ShelfBookRow[]>([])
  const selectedBookId = ref<string | null>(null)
  const activeBookId = ref<string | null>(null)
  const chapterEntries = ref<NovelChapterEntry[]>([])
  const continuityFiles = ref<NovelFileNode[]>([])
  const outlineFiles = ref<NovelFileNode[]>([])
  const volumeFiles = ref<NovelFileNode[]>([])
  const reviewFiles = ref<NovelFileNode[]>([])
  const canonFiles = ref<NovelFileNode[]>([])
  const castFiles = ref<NovelFileNode[]>([])
  const extendedState = ref<NovelExtendedState | null>(null)
  const contractRaws = ref<Record<number, string>>({})
  const reviewRaws = ref<Record<number, string>>({})
  const batchFreezeFrozen = ref(false)
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

  async function loadBatchFreezeStatus(bookId: string) {
    if (extendedState.value?.batchFreezeArtifact === 'frozen') {
      batchFreezeFrozen.value = true
      return
    }
    try {
      const raw = await readFile(novelBatchFreezePath(bookId))
      batchFreezeFrozen.value = parseBatchFreezeYaml(raw).status === 'frozen'
    } catch {
      batchFreezeFrozen.value = Boolean(extendedState.value?.frozenBatch?.from)
    }
  }

  async function loadChapterMeta(bookId: string, entries: NovelChapterEntry[]) {
    const contracts: Record<number, string> = {}
    const reviews: Record<number, string> = {}
    await Promise.all(
      entries.map(async (e) => {
        if (e.contract) {
          try {
            contracts[e.chapter] = await readFile(chapterNodePath(bookId, e.contract))
          } catch {
            /* ignore */
          }
        }
        if (e.prose) {
          try {
            reviews[e.chapter] = await readFile(novelChapterReviewPath(bookId, e.chapter))
          } catch {
            /* ignore */
          }
        }
      }),
    )
    contractRaws.value = contracts
    reviewRaws.value = reviews
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
      const chNodes = await listDirSoft(novelChaptersDir(bookId))
      const outNodes = await listDirSoft(novelOutlineDir(bookId))
      const entries = buildChapterEntries(chNodes, outNodes)
      const withContract = entries.filter((e) => Boolean(e.contract || e.prose)).length
      const total = Math.max(withContract, state.lastCommittedCh)
      return { committed: state.lastCommittedCh, total }
    } catch {
      return { committed: state.lastCommittedCh, total: Math.max(state.lastCommittedCh, 1) }
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
      const [chNodes, contNodes, outNodes, volNodes, revNodes, canonNodes, castNodes] =
        await Promise.all([
          listDirSoft(novelChaptersDir(bookId)),
          listDirSoft(novelContinuityDir(bookId)),
          listDirSoft(novelOutlineDir(bookId)),
          listDirSoft(novelVolumesDir(bookId)),
          listDirSoft(novelReviewsDir(bookId)),
          listDirSoft(novelCanonDir(bookId)),
          listDirSoft(novelCastDir(bookId)),
        ])
      chapterEntries.value = buildChapterEntries(chNodes, outNodes)
      continuityFiles.value = sortWorkbenchDocNodes(contNodes)
      outlineFiles.value = sortWorkbenchDocNodes(outNodes).filter((n) => !isNovelContractName(n.name))
      volumeFiles.value = sortWorkbenchDocNodes(volNodes)
      reviewFiles.value = sortWorkbenchDocNodes(revNodes)
      canonFiles.value = sortWorkbenchDocNodes(canonNodes)
      castFiles.value = sortWorkbenchDocNodes(castNodes)
      await loadBatchFreezeStatus(bookId)
      await loadChapterMeta(bookId, chapterEntries.value)
      await loadOutlinePreviews(bookId)
      void persistActiveBook(bookId)
    } catch {
      chapterEntries.value = []
      continuityFiles.value = []
      outlineFiles.value = []
      volumeFiles.value = []
      reviewFiles.value = []
      canonFiles.value = []
      castFiles.value = []
      extendedState.value = null
      contractRaws.value = {}
      reviewRaws.value = {}
      batchFreezeFrozen.value = false
      bookState.value = null
      bookOutlineRows.value = []
      volumeUnitRows.value = {}
    } finally {
      loading.value = false
    }
  }

  function clearBook() {
    selectedBookId.value = null
    chapterEntries.value = []
    continuityFiles.value = []
    outlineFiles.value = []
    volumeFiles.value = []
    reviewFiles.value = []
    canonFiles.value = []
    castFiles.value = []
    extendedState.value = null
    contractRaws.value = {}
    reviewRaws.value = {}
    batchFreezeFrozen.value = false
    bookState.value = null
    bookOutlineRows.value = []
    volumeUnitRows.value = {}
  }

  return {
    loading,
    books,
    selectedBookId,
    activeBookId,
    chapterEntries,
    continuityFiles,
    outlineFiles,
    volumeFiles,
    reviewFiles,
    canonFiles,
    castFiles,
    extendedState,
    contractRaws,
    reviewRaws,
    batchFreezeFrozen,
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
    chapterNodePath,
    volumeNodePath: (bookId: string, node: NovelFileNode) =>
      volumeNodePath(bookId, node, volumeFiles.value),
  }
}
