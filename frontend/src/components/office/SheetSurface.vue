<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { fetchJSON } from '@/api/client'
import { toast } from '@/utils/feedback'
import type { OfficeEditScope } from '@/utils/office-route'

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

const isJson = () => props.path.toLowerCase().endsWith('.danmo-sheet.json')
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
    if (isJson()) {
      const data = JSON.parse(fc.content || '{"sheets":[{"name":"Sheet1","rows":[[""]]}]}') as {
        sheets?: Array<{ name?: string; rows?: string[][]; colWidths?: number[] }>
      }
      const parsed = (data.sheets?.length ? data.sheets : [{ name: 'Sheet1', rows: [['']] }]).map((s, i) => ({
        name: s.name || `Sheet${i + 1}`,
        rows: s.rows?.length ? s.rows : [['']],
        colWidths: s.colWidths,
      }))
      sheets.value = parsed
      sheetIndex.value = Math.min(sheetIndex.value, parsed.length - 1)
    } else {
      const table = parseCsv(fc.content || '')
      sheets.value = [{ name: 'Sheet1', rows: table }]
      sheetIndex.value = 0
    }
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
    const content = isJson()
      ? JSON.stringify(
          {
            sheets: sheets.value.map((s) => ({
              name: s.name,
              rows: s.rows,
              colWidths: s.colWidths,
            })),
          },
          null,
          2,
        ) + '\n'
      : toCsv(rows.value)
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

function addSheet() {
  if (!isJson() || readonly.value) return
  const name = `Sheet${sheets.value.length + 1}`
  sheets.value = [...sheets.value, { name, rows: [['']], colWidths: [120] }]
  sheetIndex.value = sheets.value.length - 1
  markDirty()
}

function selectSheet(i: number) {
  sheetIndex.value = i
  selectedCell.value = null
}

function getEditScope(): OfficeEditScope {
  return 'sheet'
}

function getSelectionMarkdown(): string {
  const header = rows.value[0] || []
  const body = rows.value.slice(1)
  const line = (cells: string[]) => `| ${cells.map((c) => c.replace(/\|/g, '\\|')).join(' | ')} |`
  const sep = `| ${header.map(() => '---').join(' | ')} |`
  return [line(header), sep, ...body.map(line)].join('\n') + '\n'
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
        <button v-if="isJson()" class="sheet-surface__btn" :disabled="readonly" @click="addSheet">
          + {{ t('office.sheetTab') }}
        </button>
      </div>

      <div v-if="isJson() && sheets.length > 1" class="sheet-surface__tabs">
        <button
          v-for="(s, i) in sheets"
          :key="i"
          type="button"
          class="sheet-surface__tab"
          :class="{ 'is-active': i === sheetIndex }"
          @click="selectSheet(i)"
        >
          {{ s.name }}
        </button>
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
                  'is-selected': selectedCell?.r === ri && selectedCell?.c === ci - 1,
                  'is-number': cellLooksNumber(row[ci - 1] || ''),
                }"
                :style="{ width: `${colWidths[ci - 1]}px`, minWidth: `${colWidths[ci - 1]}px` }"
                @mousedown="selectedCell = { r: ri, c: ci - 1 }"
              >
                <input
                  class="sheet-surface__cell"
                  :value="row[ci - 1] || ''"
                  :readonly="readonly"
                  @input="updateCell(ri, ci - 1, ($event.target as HTMLInputElement).value)"
                  @focus="selectedCell = { r: ri, c: ci - 1 }"
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
  font-size: 13px;
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
  font-size: 12px;
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
  font-size: 12px;
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
  font-size: 11px;
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
.sheet-surface__grid td.is-number .sheet-surface__cell {
  text-align: right;
  font-variant-numeric: tabular-nums;
  color: var(--dq-label-primary);
}
.sheet-surface__cell {
  width: 100%;
  border: 0;
  padding: 6px 8px;
  font-size: 13px;
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
