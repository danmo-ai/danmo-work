<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { fetchJSON } from '@/api/client'
import { toast } from '@/utils/feedback'

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
const rows = ref<string[][]>([['']])
const dirty = ref(false)
const isJson = () => props.path.toLowerCase().endsWith('.danmo-sheet.json')

function parseCsv(text: string): string[][] {
  const lines = text.replace(/^\uFEFF/, '').split(/\r?\n/)
  const out: string[][] = []
  for (const line of lines) {
    if (line === '' && out.length === 0) continue
    // Minimal CSV: split on commas not inside quotes.
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

async function load() {
  if (!props.projectId || !props.path) return
  loading.value = true
  try {
    const fc = await fetchJSON<{ content: string }>(
      `/projects/${props.projectId}/files/content?path=${encodeURIComponent(props.path)}`,
    )
    if (isJson()) {
      const data = JSON.parse(fc.content || '{"sheets":[{"rows":[[""]]}]}') as {
        sheets?: Array<{ rows?: string[][] }>
      }
      rows.value = data.sheets?.[0]?.rows?.length ? data.sheets[0].rows! : [['']]
    } else {
      rows.value = parseCsv(fc.content || '')
    }
    dirty.value = false
    emit('dirty', false)
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('office.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function save() {
  if (!props.projectId) return
  saving.value = true
  try {
    const content = isJson()
      ? JSON.stringify({ sheets: [{ name: 'Sheet1', rows: rows.value }] }, null, 2) + '\n'
      : toCsv(rows.value)
    await fetchJSON(`/projects/${props.projectId}/files/content`, {
      method: 'PUT',
      body: JSON.stringify({ path: props.path, content }),
    })
    dirty.value = false
    emit('dirty', false)
    emit('saved')
    toast.success(t('office.saved'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('office.saveFailed'))
  } finally {
    saving.value = false
  }
}

function updateCell(r: number, c: number, value: string) {
  const next = rows.value.map((row) => [...row])
  while (next.length <= r) next.push([''])
  while (next[r].length <= c) next[r].push('')
  next[r][c] = value
  rows.value = next
  dirty.value = true
  emit('dirty', true)
}

function addRow() {
  const cols = Math.max(1, ...rows.value.map((r) => r.length))
  rows.value = [...rows.value, Array.from({ length: cols }, () => '')]
  dirty.value = true
  emit('dirty', true)
}

function addCol() {
  rows.value = rows.value.map((r) => [...r, ''])
  dirty.value = true
  emit('dirty', true)
}

function getSelectionMarkdown(): string {
  const header = rows.value[0] || []
  const body = rows.value.slice(1)
  const line = (cells: string[]) => `| ${cells.map((c) => c.replace(/\|/g, '\\|')).join(' | ')} |`
  const sep = `| ${header.map(() => '---').join(' | ')} |`
  return [line(header), sep, ...body.map(line)].join('\n') + '\n'
}

watch(
  () => [props.projectId, props.path, props.reloadToken] as const,
  () => load(),
  { immediate: true },
)

defineExpose({ save, getSelectionMarkdown, dirty, saving, loading })
</script>

<template>
  <div class="sheet-surface">
    <div v-if="loading" class="sheet-surface__status">{{ t('office.loading') }}</div>
    <template v-else>
      <div class="sheet-surface__toolbar">
        <button class="sheet-surface__btn" :disabled="mode === 'view' || turnRunning" @click="addRow">
          {{ t('office.addRow') }}
        </button>
        <button class="sheet-surface__btn" :disabled="mode === 'view' || turnRunning" @click="addCol">
          {{ t('office.addCol') }}
        </button>
      </div>
      <div class="sheet-surface__grid-wrap">
        <table class="sheet-surface__grid">
          <tbody>
            <tr v-for="(row, ri) in rows" :key="ri">
              <td v-for="(cell, ci) in row" :key="ci">
                <input
                  class="sheet-surface__cell"
                  :value="cell"
                  :readonly="mode === 'view' || turnRunning"
                  @input="updateCell(ri, ci, ($event.target as HTMLInputElement).value)"
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
  background: var(--dq-bg, #fff);
}
.sheet-surface__status {
  padding: 24px;
  font-size: 13px;
  color: #6b7280;
}
.sheet-surface__toolbar {
  display: flex;
  gap: 6px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--dq-border, #e5e7eb);
}
.sheet-surface__btn {
  height: 28px;
  padding: 0 10px;
  border: 1px solid var(--dq-border, #e5e7eb);
  border-radius: 6px;
  background: var(--dq-bg-subtle, #f9fafb);
  font-size: 12px;
  cursor: pointer;
}
.sheet-surface__btn:disabled {
  opacity: 0.5;
}
.sheet-surface__grid-wrap {
  flex: 1;
  overflow: auto;
  padding: 8px;
}
.sheet-surface__grid {
  border-collapse: collapse;
  min-width: 100%;
}
.sheet-surface__grid td {
  border: 1px solid var(--dq-border, #e5e7eb);
  padding: 0;
  min-width: 96px;
}
.sheet-surface__cell {
  width: 100%;
  border: 0;
  padding: 6px 8px;
  font-size: 13px;
  background: transparent;
  color: inherit;
  outline: none;
}
.sheet-surface__cell:focus {
  background: #eff6ff;
}
</style>
