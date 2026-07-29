<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { asArray, fetchJSON, isNotFoundError } from '@/api/client'
import { confirm, toast } from '@/utils/feedback'

export interface TableInfo {
  scope: string
  scopeId: string
  table: string
  count: number
}

export interface TableRow {
  scope: string
  scopeId: string
  table: string
  key: string
  data: Record<string, unknown>
  createdAt?: string
  updatedAt?: string
}

const props = defineProps<{
  projectId: string | null
  agentId: string | null
}>()

const { t } = useI18n()
const tables = ref<TableInfo[]>([])
const rows = ref<TableRow[]>([])
const loading = ref(false)
const loadingRows = ref(false)
const filter = ref('')
const selectedKey = ref<string | null>(null)
const editingKey = ref<string | null>(null)
const editJson = ref('')
const saving = ref(false)

const selected = computed(() => tables.value.find((x) => tableId(x) === selectedKey.value) || null)

const filteredTables = computed(() => {
  const q = filter.value.trim().toLowerCase()
  if (!q) return tables.value
  return tables.value.filter((x) => {
    const hay = `${x.table} ${x.scope} ${x.scopeId}`.toLowerCase()
    return hay.includes(q)
  })
})

function tableId(x: TableInfo) {
  return `${x.scope}:${x.scopeId}:${x.table}`
}

function scopeLabel(scope: string) {
  if (scope === 'user') return t('tables.scopeUser')
  if (scope === 'project') return t('tables.scopeProject')
  if (scope === 'agent') return t('tables.scopeAgent')
  return scope
}

function formatTime(iso?: string) {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleString()
}

async function loadTables() {
  loading.value = true
  try {
    const qs = new URLSearchParams()
    if (props.projectId) qs.set('projectId', props.projectId)
    if (props.agentId) qs.set('agentId', props.agentId)
    const q = qs.toString()
    tables.value = asArray(await fetchJSON<TableInfo[]>(`/tables${q ? `?${q}` : ''}`))
    if (selectedKey.value && !tables.value.some((x) => tableId(x) === selectedKey.value)) {
      selectedKey.value = null
      rows.value = []
    }
  } catch (e) {
    tables.value = []
    if (!isNotFoundError(e)) {
      toast.error(e instanceof Error ? e.message : t('tables.loadFailed'))
    }
  } finally {
    loading.value = false
  }
}

async function loadRows(info: TableInfo) {
  loadingRows.value = true
  editingKey.value = null
  try {
    const qs = new URLSearchParams({
      scope: info.scope,
      scopeId: info.scopeId,
      limit: '100',
    })
    rows.value = asArray(
      await fetchJSON<TableRow[]>(
        `/tables/${encodeURIComponent(info.table)}/rows?${qs.toString()}`,
      ),
    )
  } catch (e) {
    rows.value = []
    toast.error(e instanceof Error ? e.message : t('tables.loadFailed'))
  } finally {
    loadingRows.value = false
  }
}

async function selectTable(info: TableInfo) {
  selectedKey.value = tableId(info)
  await loadRows(info)
}

function startEdit(row: TableRow) {
  editingKey.value = row.key
  editJson.value = JSON.stringify(row.data ?? {}, null, 2)
}

function cancelEdit() {
  editingKey.value = null
  editJson.value = ''
}

async function saveRow(row: TableRow) {
  let data: Record<string, unknown>
  try {
    data = JSON.parse(editJson.value) as Record<string, unknown>
    if (!data || typeof data !== 'object' || Array.isArray(data)) throw new Error('object required')
  } catch {
    toast.error(t('tables.invalidJson'))
    return
  }
  saving.value = true
  try {
    const qs = new URLSearchParams({
      scope: row.scope,
      scopeId: row.scopeId,
    })
    const saved = await fetchJSON<TableRow>(
      `/tables/${encodeURIComponent(row.table)}/rows/${encodeURIComponent(row.key)}?${qs.toString()}`,
      {
        method: 'PUT',
        body: JSON.stringify({ data }),
      },
    )
    rows.value = rows.value.map((r) => (r.key === row.key ? { ...r, ...saved, data } : r))
    editingKey.value = null
    toast.success(t('tables.saved'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('tables.saveFailed'))
  } finally {
    saving.value = false
  }
}

async function removeRow(row: TableRow) {
  try {
    await confirm(`${row.table} / ${row.key}`, t('tables.delete'), { type: 'warning' })
  } catch {
    return
  }
  try {
    const qs = new URLSearchParams({
      scope: row.scope,
      scopeId: row.scopeId,
    })
    await fetchJSON(
      `/tables/${encodeURIComponent(row.table)}/rows/${encodeURIComponent(row.key)}?${qs.toString()}`,
      { method: 'DELETE' },
    )
    rows.value = rows.value.filter((r) => r.key !== row.key)
    if (selected.value) {
      selected.value.count = Math.max(0, (selected.value.count || 1) - 1)
    }
    toast.success(t('tables.deleted'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('tables.deleteFailed'))
  }
}

function previewData(data: Record<string, unknown>) {
  try {
    const s = JSON.stringify(data)
    return s.length > 120 ? `${s.slice(0, 117)}…` : s
  } catch {
    return ''
  }
}

async function refresh() {
  await loadTables()
  if (selected.value) await loadRows(selected.value)
}

onMounted(() => {
  void loadTables()
})

watch(
  () => [props.projectId, props.agentId] as const,
  () => {
    void loadTables()
  },
)

defineExpose({ refresh })
</script>

<template>
  <div class="tables-panel">
    <div class="tables-panel__header">
      <span class="tables-panel__title">{{ t('tables.title') }}</span>
      <button type="button" class="tables-panel__btn" :disabled="loading" @click="refresh">
        {{ t('tables.refresh') }}
      </button>
    </div>

    <input
      v-model="filter"
      class="tables-panel__filter"
      type="search"
      :placeholder="t('tables.filterPlaceholder')"
    />

    <div v-if="loading" class="tables-panel__status">{{ t('tables.loading') }}</div>
    <DqEmpty
      v-else-if="!filteredTables.length"
      class="tables-panel__empty"
      :description="t('tables.empty')"
    >
      <p class="tables-panel__hint">{{ t('tables.emptyHint') }}</p>
    </DqEmpty>

    <template v-else>
      <div class="tables-panel__list">
        <button
          v-for="info in filteredTables"
          :key="tableId(info)"
          type="button"
          class="tables-panel__table"
          :class="{ 'is-active': selectedKey === tableId(info) }"
          @click="selectTable(info)"
        >
          <span class="tables-panel__table-name">{{ info.table }}</span>
          <span class="tables-panel__table-meta">
            {{ scopeLabel(info.scope) }} · {{ t('tables.rows', { n: info.count }) }}
          </span>
        </button>
      </div>

      <div class="tables-panel__rows">
        <div v-if="!selected" class="tables-panel__status">{{ t('tables.selectTable') }}</div>
        <div v-else-if="loadingRows" class="tables-panel__status">{{ t('tables.loading') }}</div>
        <div v-else-if="!rows.length" class="tables-panel__status">{{ t('tables.noRows') }}</div>
        <div v-else class="tables-panel__row-list">
          <article v-for="row in rows" :key="row.key" class="tables-panel__row">
            <div class="tables-panel__row-head">
              <code class="tables-panel__key">{{ row.key }}</code>
              <span class="tables-panel__time">{{ formatTime(row.updatedAt) }}</span>
            </div>
            <template v-if="editingKey === row.key">
              <textarea v-model="editJson" class="tables-panel__json" rows="8" spellcheck="false" />
              <div class="tables-panel__actions">
                <button type="button" class="tables-panel__btn" :disabled="saving" @click="saveRow(row)">
                  {{ t('tables.save') }}
                </button>
                <button type="button" class="tables-panel__btn" @click="cancelEdit">
                  {{ t('tables.cancel') }}
                </button>
              </div>
            </template>
            <template v-else>
              <pre class="tables-panel__preview">{{ previewData(row.data) }}</pre>
              <div class="tables-panel__actions">
                <button type="button" class="tables-panel__btn" @click="startEdit(row)">
                  {{ t('tables.edit') }}
                </button>
                <button type="button" class="tables-panel__btn tables-panel__btn--danger" @click="removeRow(row)">
                  {{ t('tables.delete') }}
                </button>
              </div>
            </template>
          </article>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.tables-panel {
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
  padding: 8px;
  gap: 8px;
}
.tables-panel__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.tables-panel__title {
  font-size: 13px;
  font-weight: 600;
  color: var(--dq-label-primary);
}
.tables-panel__btn {
  height: 26px;
  padding: 0 8px;
  border: 1px solid var(--dq-border);
  border-radius: 5px;
  background: var(--dq-fill-tertiary);
  color: var(--dq-label-primary);
  font-size: 12px;
  cursor: pointer;
}
.tables-panel__btn:hover:not(:disabled) {
  background: color-mix(in srgb, var(--dq-label-primary) 8%, var(--dq-fill-tertiary));
}
.tables-panel__btn:disabled {
  opacity: 0.5;
}
.tables-panel__btn--danger {
  color: var(--dq-danger, #c0392b);
}
.tables-panel__filter {
  height: 30px;
  border: 1px solid var(--dq-border);
  border-radius: 6px;
  padding: 0 8px;
  background: var(--dq-bg-base);
  color: var(--dq-label-primary);
  font-size: 12px;
  outline: none;
}
.tables-panel__status,
.tables-panel__hint {
  font-size: 12px;
  color: var(--dq-label-tertiary);
  padding: 8px 4px;
}
.tables-panel__empty {
  margin-top: 24px;
}
.tables-panel__list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 36%;
  overflow: auto;
  flex-shrink: 0;
}
.tables-panel__table {
  text-align: left;
  border: 1px solid var(--dq-border);
  border-radius: 6px;
  padding: 7px 8px;
  background: color-mix(in srgb, var(--dq-bg-elevated) 40%, transparent);
  color: var(--dq-label-primary);
  cursor: pointer;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.tables-panel__table.is-active {
  border-color: var(--dq-accent);
  box-shadow: inset 2px 0 0 var(--dq-accent);
}
.tables-panel__table-name {
  font-size: 12px;
  font-weight: 600;
}
.tables-panel__table-meta {
  font-size: 11px;
  color: var(--dq-label-tertiary);
}
.tables-panel__rows {
  flex: 1;
  min-height: 0;
  overflow: auto;
  border-top: 1px solid var(--dq-separator-light);
  padding-top: 8px;
}
.tables-panel__row-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.tables-panel__row {
  border: 1px solid var(--dq-border);
  border-radius: 6px;
  padding: 8px;
  background: color-mix(in srgb, var(--dq-bg-elevated) 30%, transparent);
}
.tables-panel__row-head {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 6px;
}
.tables-panel__key {
  font-size: 12px;
  color: var(--dq-accent);
}
.tables-panel__time {
  font-size: 11px;
  color: var(--dq-label-tertiary);
}
.tables-panel__preview,
.tables-panel__json {
  width: 100%;
  margin: 0;
  padding: 6px 8px;
  border-radius: 5px;
  border: 1px solid var(--dq-border);
  background: color-mix(in srgb, var(--dq-fill-tertiary) 60%, transparent);
  color: var(--dq-label-secondary);
  font-family: var(--dq-font-mono, ui-monospace, Menlo, monospace);
  font-size: 11px;
  line-height: 1.45;
  white-space: pre-wrap;
  word-break: break-word;
  box-sizing: border-box;
}
.tables-panel__json {
  resize: vertical;
  color: var(--dq-label-primary);
  outline: none;
}
.tables-panel__actions {
  display: flex;
  gap: 6px;
  margin-top: 6px;
}
</style>
