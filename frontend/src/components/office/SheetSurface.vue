<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { fetchJSON } from '@/api/client'
import { toast } from '@/utils/feedback'
import type { FileEditScope } from '@/utils/file-route'

interface SheetTab {
  name: string
  rows: string[][]
  /** Column widths in px; parallel to max columns. */
  colWidths?: number[]
}

const props = defineProps<{
  projectId: string
  path: string
  mode: 'view' | 'edit' | 'present'
  reloadToken: number
  turnRunning?: boolean
}>()

const emit = defineEmits<{
  dirty: [value: boolean]
  saved: []
}>()

const { t } = useI18n()
const loading = ref(false)
const saving = ref(false)
const sheets = ref<SheetTab[]>([{ name: 'Sheet1', rows: [['']], colWidths: [120] }])
const sheetIndex = ref(0)
const dirty = ref(false)
const gridRef = ref<HTMLElement | null>(null)
const scrollTop = ref(0)
const scrollLeft = ref(0)
const selectedCell = ref<{ r: number; c: number } | null>(null)
/** Inclusive selection range (anchor → focus). When null, falls back to whole sheet for AI. */
const selectionRange = ref<{ r0: number; c0: number; r1: number; c1: number } | null>(null)

const readonly = computed(() => props.mode === 'view' || !!props.turnRunning)

const activeSheet = computed(() => sheets.value[sheetIndex.value] || sheets.value[0])
const rows = computed(() => activeSheet.value?.rows || [['']])
const colCount = computed(() => Math.max(1, ...rows.value.map((r) => r.length)))
const colWidths = computed(() => {
  const widths = activeSheet.value?.colWidths || []
  return Array.from({ length: colCount.value }, (_, i) => widths[i] || 120)
})

function colLabel(i: number): string {
  let n = i
  let s = ''
  do {
    s = String.fromCharCode(65 + (n % 26)) + s
    n = Math.floor(n / 26) - 1
  } while (n >= 0)
  return s
}

function cellLooksNumber(value: string): boolean {
  if (!value.trim()) return false
  return /^-?\d+(\.\d+)?([eE][+-]?\d+)?%$/.test(value.trim()) || /^-?\d+(\.\d+)?$/.test(value.trim())
}

function cellLooksFormula(value: string): boolean {
  return value.trimStart().startsWith('=')
}

function normalizeRange(r0: number, c0: number, r1: number, c1: number) {
  return {
    r0: Math.min(r0, r1),
    c0: Math.min(c0, c1),
    r1: Math.max(r0, r1),
    c1: Math.max(c0, c1),
  }
}

function isInSelection(r: number, c: number): boolean {
  const sel = selectionRange.value
  if (!sel) return selectedCell.value?.r === r && selectedCell.value?.c === c
  return r >= sel.r0 && r <= sel.r1 && c >= sel.c0 && c <= sel.c1
}

function selectCell(r: number, c: number, ev?: MouseEvent) {
  if (ev?.shiftKey && selectedCell.value) {
    selectionRange.value = normalizeRange(selectedCell.value.r, selectedCell.value.c, r, c)
  } else {
    selectedCell.value = { r, c }
    selectionRange.value = { r0: r, c0: c, r1: r, c1: c }
  }
}

function a1(r: number, c: number): string {
  return `${colLabel(c)}${r + 1}`
}

/** Fill selected column(s) downward from the top row of the selection. */
function fillDown() {
  if (readonly.value || !selectionRange.value) return
  const { r0, c0, r1, c1 } = selectionRange.value
  if (r1 <= r0) return
  mutateActive((sheet) => {
    const nextRows = sheet.rows.map((row) => [...row])
    for (let c = c0; c <= c1; c++) {
      const src = nextRows[r0]?.[c] ?? ''
      for (let r = r0 + 1; r <= r1; r++) {
        while (nextRows.length <= r) nextRows.push([''])
        while (nextRows[r].length <= c) nextRows[r].push('')
        nextRows[r][c] = src
      }
    }
    return { ...sheet, rows: nextRows }
  })
}

function parseCsv(text: string): string[][] {
  const lines = text.replace(/^\uFEFF/, '').split(/\r?\n/)
  const out: string[][] = []
  for (const line of lines) {
    if (line === '' && out.length === 0) continue
    const cells: string[] = []
    let cur = ''
    let inQ = false
    for (let i = 0; i < line.length; i++) {
      const ch = line[i]
      if (ch === '"') {
        if (inQ && line[i + 1] === '"') {
          cur += '"'
          i++
        } else inQ = !inQ
      } else if (ch === ',' && !inQ) {
        cells.push(cur)
        cur = ''
      } else cur += ch
    }
    cells.push(cur)
    out.push(cells)
  }
  return out.length ? out : [['']]
}

function toCsv(table: string[][]): string {
  return (
    table
      .map((row) =>
        row
          .map((cell) => {
            const s = cell ?? ''
            if (/[",\n\r]/.test(s)) return `"${s.replace(/"/g, '""')}"`
            return s
          })
          .join(','),
      )
      .join('\n') + '\n'
  )
}

function markDirty() {
  dirty.value = true
  emit('dirty', true)
}

function mutateActive(mutator: (sheet: SheetTab) => SheetTab) {
  const next = sheets.value.map((s, i) => (i === sheetIndex.value ? mutator({ ...s, rows: s.rows.map((r) => [...r]) }) : s))
  sheets.value = next
  markDirty()
}

async function load(opts?: { resetScroll?: boolean }) {
  if (!props.projectId || !props.path) return
  if (opts?.resetScroll) {
    scrollTop.value = 0
    scrollLeft.value = 0
    sheetIndex.value = 0
  } else if (gridRef.value) {
    scrollTop.value = gridRef.value.scrollTop
    scrollLeft.value = gridRef.value.scrollLeft
  }
  loading.value = true
  try {
    const fc = await fetchJSON<{ content: string }>(
      `/projects/${props.projectId}/files/content?path=${encodeURIComponent(props.path)}`,
    )
    const table = parseCsv(fc.content || '')
    sheets.value = [{ name: 'Sheet1', rows: table }]
    sheetIndex.value = 0
    dirty.value = false
    emit('dirty', false)
    selectedCell.value = null
    await nextTick()
    if (gridRef.value) {
      gridRef.value.scrollTop = scrollTop.value
      gridRef.value.scrollLeft = scrollLeft.value
    }
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('office.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function save(opts?: { quiet?: boolean }) {
  if (!props.projectId) return
  saving.value = true
  try {
    const content = toCsv(rows.value)
    await fetchJSON(`/projects/${props.projectId}/files/content`, {
      method: 'PUT',
      body: JSON.stringify({ path: props.path, content }),
    })
    dirty.value = false
    emit('dirty', false)
    emit('saved')
    if (!opts?.quiet) toast.success(t('office.saved'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('office.saveFailed'))
    throw e
  } finally {
    saving.value = false
  }
}

function updateCell(r: number, c: number, value: string) {
  mutateActive((sheet) => {
    const nextRows = sheet.rows.map((row) => [...row])
    while (nextRows.length <= r) nextRows.push([''])
    while (nextRows[r].length <= c) nextRows[r].push('')
    nextRows[r][c] = value
    return { ...sheet, rows: nextRows }
  })
}

function addRow() {
  mutateActive((sheet) => {
    const cols = Math.max(1, ...sheet.rows.map((r) => r.length))
    return { ...sheet, rows: [...sheet.rows, Array.from({ length: cols }, () => '')] }
  })
}

function addCol() {
  mutateActive((sheet) => {
    const cols = Math.max(1, ...sheet.rows.map((r) => r.length))
    const widths = [...(sheet.colWidths || [])]
    while (widths.length < cols) widths.push(120)
    widths.push(120)
    return {
      ...sheet,
      rows: sheet.rows.map((r) => [...r, '']),
      colWidths: widths,
    }
  })
}

function deleteRow() {
  const r = selectedCell.value?.r
  if (r == null) return
  mutateActive((sheet) => {
    if (sheet.rows.length <= 1) return { ...sheet, rows: [['']] }
    return { ...sheet, rows: sheet.rows.filter((_, i) => i !== r) }
  })
  selectedCell.value = null
}

function onColResize(c: number, e: MouseEvent) {
  if (readonly.value) return
  e.preventDefault()
  const startX = e.clientX
  const startW = colWidths.value[c] || 120
  function onMove(ev: MouseEvent) {
    const nextW = Math.max(64, startW + (ev.clientX - startX))
    mutateActive((sheet) => {
      const widths = Array.from({ length: Math.max(colCount.value, c + 1) }, (_, i) =>
        i === c ? nextW : sheet.colWidths?.[i] || 120,
      )
      return { ...sheet, colWidths: widths }
    })
  }
  function onUp() {
    window.removeEventListener('mousemove', onMove)
    window.removeEventListener('mouseup', onUp)
  }
  window.addEventListener('mousemove', onMove)
  window.addEventListener('mouseup', onUp)
}

function getEditScope(): FileEditScope {
  return 'sheet'
}

function getSelectionMarkdown(): string {
  const sel = selectionRange.value
  const line = (cells: string[]) => `| ${cells.map((c) => (c ?? '').replace(/\|/g, '\\|')).join(' | ')} |`
  if (sel && (sel.r0 !== sel.r1 || sel.c0 !== sel.c1 || selectedCell.value)) {
    const { r0, c0, r1, c1 } = sel
    const width = c1 - c0 + 1
    const header = Array.from({ length: width }, (_, i) => a1(r0, c0 + i))
    const sep = `| ${header.map(() => '---').join(' | ')} |`
    const body: string[] = []
    for (let r = r0; r <= r1; r++) {
      const cells = Array.from({ length: width }, (_, i) => rows.value[r]?.[c0 + i] ?? '')
      body.push(line(cells))
    }
    return [
      `sheet: ${activeSheet.value?.name || 'Sheet1'}`,
      `range: ${a1(r0, c0)}:${a1(r1, c1)}`,
      line(header),
      sep,
      ...body,
      '',
    ].join('\n')
  }
  const header = rows.value[0] || []
  const body = rows.value.slice(1)
  const sep = `| ${header.map(() => '---').join(' | ')} |`
  return [
    `sheet: ${activeSheet.value?.name || 'Sheet1'}`,
    `range: all`,
    line(header),
    sep,
    ...body.map(line),
    '',
  ].join('\n')
}

watch(
  () => [props.projectId, props.path] as const,
  () => {
    void load({ resetScroll: true })
  },
  { immediate: true },
)

watch(
  () => props.reloadToken,
  () => {
    void load({ resetScroll: false })
  },
)

defineExpose({ save, getSelectionMarkdown, getEditScope, dirty, saving, loading })
</script>

<template>
  <div class="sheet-surface">
    <div v-if="loading" class="sheet-surface__status">{{ t('office.loading') }}</div>
    <template v-else>
      <div class="sheet-surface__toolbar">
        <button class="sheet-surface__btn" :disabled="readonly" @click="addRow">
          {{ t('office.addRow') }}
        </button>
        <button class="sheet-surface__btn" :disabled="readonly" @click="addCol">
          {{ t('office.addCol') }}
        </button>
        <button class="sheet-surface__btn" :disabled="readonly || selectedCell == null" @click="deleteRow">
          {{ t('office.deleteRow') }}
        </button>
        <button
          class="sheet-surface__btn"
          :disabled="readonly || !selectionRange || selectionRange.r1 <= selectionRange.r0"
          @click="fillDown"
        >
          {{ t('office.fillDown') }}
        </button>
        <span v-if="selectionRange" class="sheet-surface__range">
          {{ a1(selectionRange.r0, selectionRange.c0) }}:{{ a1(selectionRange.r1, selectionRange.c1) }}
        </span>
      </div>

      <div ref="gridRef" class="sheet-surface__grid-wrap">
        <table class="sheet-surface__grid">
          <thead>
            <tr>
              <th class="sheet-surface__corner" />
              <th
                v-for="ci in colCount"
                :key="ci"
                class="sheet-surface__col-head"
                :style="{ width: `${colWidths[ci - 1]}px`, minWidth: `${colWidths[ci - 1]}px` }"
              >
                <span>{{ colLabel(ci - 1) }}</span>
                <span
                  class="sheet-surface__resize"
                  :title="t('office.colWidth')"
                  @mousedown="onColResize(ci - 1, $event)"
                />
              </th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(row, ri) in rows" :key="ri">
              <th class="sheet-surface__row-head">{{ ri + 1 }}</th>
              <td
                v-for="ci in colCount"
                :key="ci"
                :class="{
                  'is-selected': isInSelection(ri, ci - 1),
                  'is-number': cellLooksNumber(row[ci - 1] || '') && !cellLooksFormula(row[ci - 1] || ''),
                  'is-formula': cellLooksFormula(row[ci - 1] || ''),
                }"
                :style="{ width: `${colWidths[ci - 1]}px`, minWidth: `${colWidths[ci - 1]}px` }"
                @mousedown="selectCell(ri, ci - 1, $event)"
              >
                <input
                  class="sheet-surface__cell"
                  :value="row[ci - 1] || ''"
                  :readonly="readonly"
                  @input="updateCell(ri, ci - 1, ($event.target as HTMLInputElement).value)"
                  @focus="selectCell(ri, ci - 1)"
                />
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </template>
  </div>
</template>

<style scoped>
.sheet-surface {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: var(--dq-bg-base);
  color: var(--dq-label-primary);
}
.sheet-surface__status {
  padding: 24px;
  font-size: var(--dq-font-size-body);
  color: var(--dq-label-tertiary);
}
.sheet-surface__toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--dq-separator-light);
  background: color-mix(in srgb, var(--dq-bg-elevated) 40%, transparent);
}
.sheet-surface__tabs {
  display: flex;
  gap: 4px;
  padding: 6px 10px;
  border-bottom: 1px solid var(--dq-separator-light);
  overflow-x: auto;
}
.sheet-surface__tab {
  height: 26px;
  padding: 0 10px;
  border: 1px solid var(--dq-border);
  border-radius: 5px;
  background: var(--dq-fill-tertiary);
  color: var(--dq-label-secondary);
  font-size: var(--dq-font-size-body);
  cursor: pointer;
}
.sheet-surface__tab.is-active {
  border-color: var(--dq-accent);
  color: var(--dq-label-primary);
  box-shadow: inset 0 -2px 0 var(--dq-accent);
}
.sheet-surface__btn {
  height: 28px;
  padding: 0 10px;
  border: 1px solid var(--dq-border);
  border-radius: 6px;
  background: var(--dq-fill-tertiary);
  color: var(--dq-label-primary);
  font-size: var(--dq-font-size-body);
  cursor: pointer;
}
.sheet-surface__btn:hover:not(:disabled) {
  background: color-mix(in srgb, var(--dq-label-primary) 8%, var(--dq-fill-tertiary));
}
.sheet-surface__btn:disabled {
  opacity: 0.5;
}
.sheet-surface__grid-wrap {
  flex: 1;
  overflow: auto;
  padding: 0;
}
.sheet-surface__grid {
  border-collapse: separate;
  border-spacing: 0;
  min-width: 100%;
}
.sheet-surface__corner,
.sheet-surface__col-head,
.sheet-surface__row-head {
  position: sticky;
  background: color-mix(in srgb, var(--dq-bg-elevated) 85%, transparent);
  color: var(--dq-label-tertiary);
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  z-index: 2;
  border-bottom: 1px solid var(--dq-border);
  border-right: 1px solid var(--dq-border);
}
.sheet-surface__corner {
  left: 0;
  top: 0;
  z-index: 3;
  min-width: 36px;
  width: 36px;
}
.sheet-surface__col-head {
  top: 0;
  height: 28px;
  position: relative;
  text-align: center;
  user-select: none;
}
.sheet-surface__row-head {
  left: 0;
  width: 36px;
  min-width: 36px;
  text-align: center;
  padding: 0 4px;
}
.sheet-surface__resize {
  position: absolute;
  top: 0;
  right: 0;
  width: 5px;
  height: 100%;
  cursor: col-resize;
}
.sheet-surface__grid td {
  border-bottom: 1px solid var(--dq-border);
  border-right: 1px solid var(--dq-border);
  padding: 0;
  background: color-mix(in srgb, var(--dq-bg-elevated) 35%, transparent);
}
.sheet-surface__grid td.is-selected {
  outline: 2px solid var(--dq-accent);
  outline-offset: -2px;
  z-index: 1;
}
.sheet-surface__grid td.is-formula .sheet-surface__cell {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  color: var(--dq-accent);
}
.sheet-surface__range {
  margin-left: 8px;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-tertiary);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}
.sheet-surface__grid td.is-number .sheet-surface__cell {
  text-align: right;
  font-variant-numeric: tabular-nums;
  color: var(--dq-label-primary);
}
.sheet-surface__cell {
  width: 100%;
  border: 0;
  padding: 6px 8px;
  font-size: var(--dq-font-size-body);
  background: transparent;
  color: var(--dq-label-primary);
  outline: none;
  box-sizing: border-box;
}
.sheet-surface__cell:focus {
  background: var(--dq-selection-bg, var(--dq-accent-tint));
}
.sheet-surface__cell:read-only {
  color: var(--dq-label-secondary);
}
</style>
