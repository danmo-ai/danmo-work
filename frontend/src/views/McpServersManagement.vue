<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import WorkspaceShell from '@/components/common/WorkspaceShell.vue'
import MarketBrowser from '@/components/market/MarketBrowser.vue'
import MarketCatalogRail from '@/components/market/MarketCatalogRail.vue'
import { useMcpServersStore } from '@/stores/mcpServers'
import { useMarketStore } from '@/stores/market'
import { confirm, toast } from '@/utils/feedback'
import type { MCPAuthMode, MCPServer, MCPToolDef } from '@/types'

type Transport = 'stdio' | 'sse' | 'streamable-http'
type PageView = 'library' | 'market'

const { t } = useI18n()
const mcp = useMcpServersStore()
const marketStore = useMarketStore()

const pageView = ref<PageView>('library')
const pageViewOptions = computed(() => [
  { label: t('market.library'), value: 'library' as const },
  { label: t('market.tab'), value: 'market' as const },
])
const selectedId = ref<string | null>(null)
const marketSelectedKey = ref<string | null>(null)
const isCreating = ref(false)
const saving = ref(false)
const refreshingTools = ref(false)
const headerSecretsText = ref('')
const accessToken = ref('')

const transportOptions: { value: Transport; label: string }[] = [
  { value: 'stdio', label: 'STDIO' },
  { value: 'sse', label: 'SSE' },
  { value: 'streamable-http', label: 'Streamable HTTP' },
]

const form = ref<MCPServer>({
  id: '',
  name: '',
  description: '',
  transport: 'stdio',
  command: '',
  args: '',
  url: '',
  env: '',
  auth: 'none',
  status: 'disconnected',
  enabled: true,
  ambientMount: true,
})

const authOptions: { value: MCPAuthMode; label: string }[] = [
  { value: 'none', label: t('connectors.authNone') },
  { value: 'headers', label: t('connectors.authHeaders') },
  { value: 'oauth', label: t('connectors.authOAuth') },
]

function parseSecrets(val: string): Record<string, string> {
  const map: Record<string, string> = {}
  for (const line of val.split('\n')) {
    const trimmed = line.trim()
    if (!trimmed) continue
    const eqIdx = trimmed.indexOf('=')
    if (eqIdx > 0) {
      map[trimmed.slice(0, eqIdx).trim()] = trimmed.slice(eqIdx + 1).trim()
    }
  }
  return map
}

/** Editable text for headers (KEY=VALUE per line) */
const headersText = computed({
  get() {
    const h = form.value.headers
    if (!h || Object.keys(h).length === 0) return ''
    return Object.entries(h).map(([k, v]) => `${k}=${v}`).join('\n')
  },
  set(val: string) {
    const map: Record<string, string> = {}
    for (const line of val.split('\n')) {
      const trimmed = line.trim()
      if (!trimmed) continue
      const eqIdx = trimmed.indexOf('=')
      if (eqIdx > 0) {
        map[trimmed.slice(0, eqIdx).trim()] = trimmed.slice(eqIdx + 1).trim()
      }
    }
    form.value.headers = map
  },
})

/** Discovered tools list from the selected server */
const discoveredTools = computed<MCPToolDef[]>(() => {
  if (!selected.value?.discoveredTools) return []
  return selected.value.discoveredTools
})

const sortedServers = computed(() =>
  [...mcp.items].sort((a, b) => a.name.localeCompare(b.name, 'zh-CN')),
)
const selected = computed(() => mcp.items.find((s) => s.id === selectedId.value))
const marketSelected = computed(() => {
  if (!marketSelectedKey.value) return null
  return (
    marketStore.catalog.find(
      (item) => item.kind === 'connector' && `${item.sourceId}:${item.id}` === marketSelectedKey.value,
    ) ?? null
  )
})
const hasSelection = computed(
  () =>
    (pageView.value === 'market' && !!marketSelectedKey.value) ||
    isCreating.value ||
    !!selectedId.value,
)
const headerTitle = computed(() => {
  if (pageView.value === 'market') {
    return marketSelected.value?.name || t('market.tab')
  }
  if (isCreating.value) return form.value.name.trim() || t('connectors.newServer')
  return selected.value?.name.trim() || t('connectors.untitled')
})

async function refreshLibrary(preferSelectId?: string | null) {
  await mcp.load()
  const prefer = preferSelectId ? resolveServerId(preferSelectId) : null
  if (prefer) {
    selectServer(prefer)
    return
  }
  if (selectedId.value && mcp.items.some((s) => s.id === selectedId.value)) {
    selectServer(selectedId.value)
    return
  }
  if (sortedServers.value.length) {
    selectServer(sortedServers.value[0].id)
  } else {
    selectedId.value = null
  }
}

onMounted(() => {
  void refreshLibrary()
})

watch(pageView, (view) => {
  if (view === 'library') {
    void refreshLibrary(selectedId.value)
  }
})

function selectServer(id: string) {
  isCreating.value = false
  selectedId.value = id
  const server = mcp.items.find((s) => s.id === id)
  if (server) form.value = { ...server }
}

function resolveServerId(catalogOrServerId: string): string | null {
  if (!catalogOrServerId) return null
  if (mcp.items.some((s) => s.id === catalogOrServerId)) return catalogOrServerId
  const byCatalog = mcp.items.find((s) => s.catalogId === catalogOrServerId)
  return byCatalog?.id ?? null
}

async function onMarketInstalled(id: string) {
  pageView.value = 'library'
  await refreshLibrary(id)
}

async function viewInstalledConnector(id: string) {
  pageView.value = 'library'
  await refreshLibrary(id)
}

async function onMarketUninstalled() {
  await mcp.load()
  if (selectedId.value && !mcp.items.some((s) => s.id === selectedId.value)) {
    selectedId.value = null
  }
}

function openCreate() {
  pageView.value = 'library'
  isCreating.value = true
  selectedId.value = null
  form.value = {
    id: '',
    name: '',
    description: '',
    transport: 'stdio',
    command: '',
    args: '',
    url: '',
    env: '',
    auth: 'none',
    status: 'disconnected',
    enabled: true,
    ambientMount: true,
  }
  headerSecretsText.value = ''
  accessToken.value = ''
}

async function save() {
  if (!form.value.name.trim()) {
    toast.warning(t('connectors.namePlaceholder'))
    return
  }
  saving.value = true
  try {
    const headerSecrets = parseSecrets(headerSecretsText.value)
    const payload = { ...form.value, name: form.value.name.trim(), headerSecrets }
    if (isCreating.value) {
      const server = await mcp.create(payload)
      toast.success(t('connectors.created'))
      isCreating.value = false
      headerSecretsText.value = ''
      selectServer(server.id)
    } else if (selected.value) {
      await mcp.update(selected.value.id, payload)
      toast.success(t('connectors.saved'))
      headerSecretsText.value = ''
      selectServer(selected.value.id)
    }
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('common.saveFailed'))
  } finally {
    saving.value = false
  }
}

async function saveOAuthToken() {
  if (!selected.value || !accessToken.value.trim()) return
  try {
    await mcp.completeOAuth(selected.value.id, { accessToken: accessToken.value.trim() })
    accessToken.value = ''
    toast.success(t('connectors.oauthSaved'))
    selectServer(selected.value.id)
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('common.saveFailed'))
  }
}

async function removeSelected() {
  if (!selected.value) return
  try {
    await confirm(t('connectors.deleteConfirm', { name: selected.value.name }), t('connectors.deleteTitle'), { type: 'warning' })
  } catch {
    return
  }
  await mcp.remove(selected.value.id)
  selectedId.value = null
  isCreating.value = false
  toast.success(t('connectors.deleted'))
}

async function toggleEnabled() {
  if (!selected.value) return
  const next = !selected.value.enabled
  await mcp.update(selected.value.id, { enabled: next })
  selectServer(selected.value.id)
}

async function handleRefreshTools() {
  if (!selected.value || refreshingTools.value) return
  refreshingTools.value = true
  try {
    await mcp.refreshTools(selected.value.id)
    selectServer(selected.value.id)
    toast.success(t('connectors.toolsRefreshed'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('connectors.refreshToolsFailed'))
  } finally {
    refreshingTools.value = false
  }
}

async function handleToggleTool(toolName: string, enabled: boolean) {
  if (!selected.value) return
  try {
    await mcp.toggleTool(selected.value.id, toolName, enabled)
    selectServer(selected.value.id)
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('common.saveFailed'))
  }
}

function initial(name: string) {
  return name.trim().charAt(0).toUpperCase() || 'M'
}

function onKeydown(e: KeyboardEvent) {
  if (pageView.value !== 'library') return
  if ((e.metaKey || e.ctrlKey) && e.key === 's') {
    e.preventDefault()
    save()
  }
}
</script>

<template>
  <WorkspaceShell
    custom-rail
    :has-selection="hasSelection"
    @create="openCreate"
    @keydown="onKeydown"
  >
    <template #rail>
      <div class="resource-rail__section">
        <div class="resource-rail__section-head">
          <DqSegmented v-model="pageView" block class="resource-rail__page-view" :options="pageViewOptions" />
        </div>
        <template v-if="pageView === 'library'">
          <div class="resource-rail__section-head">
            <span class="resource-rail__section-title">{{ $t('connectors.serverList') }}</span>
            <DqIconButton :aria-label="$t('connectors.newServer')" @click="openCreate">
              <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M12 5v14M5 12h14" stroke-linecap="round" />
              </svg>
            </DqIconButton>
          </div>
          <DqEmpty v-if="!sortedServers.length" class="resource-rail__empty" :description="$t('connectors.noServers')" />
          <nav v-else class="resource-rail__list" :aria-label="$t('connectors.serverList')">
            <button
              v-for="server in sortedServers"
              :key="server.id"
              type="button"
              class="resource-rail__row"
              :class="{ 'is-active': selectedId === server.id && !isCreating }"
              @click="selectServer(server.id)"
            >
              <span class="resource-rail__avatar">{{ initial(server.name) }}</span>
              <span class="resource-rail__meta">
                <span class="resource-rail__name">{{ server.name }}</span>
                <span class="resource-rail__desc">{{ server.transport }}</span>
              </span>
              <span
                class="resource-rail__tag"
                :class="server.status === 'connected' ? 'is-accent' : ''"
              >
                {{ server.status === 'connected' ? $t('connectors.connected') : $t('connectors.notConnected') }}
              </span>
            </button>
          </nav>
        </template>
        <MarketCatalogRail v-else v-model:selected-key="marketSelectedKey" kind="connector" />
      </div>
    </template>

    <template #empty>
      <DqEmpty :description="$t('connectors.emptySelection')">
        <p class="resource-workspace__hint">{{ $t('connectors.emptySelectionHint') }}</p>
        <DqButton @click="pageView = 'market'">{{ $t('market.tab') }}</DqButton>
      </DqEmpty>
    </template>

    <template #header>
      <div class="resource-workspace__identity">
        <h1 class="resource-workspace__title">{{ headerTitle }}</h1>
        <div v-if="pageView === 'library' && !isCreating && selected" class="resource-workspace__badges">
          <span class="resource-status" :class="`resource-status--${selected.status}`">
            <span class="resource-status__dot" />
            {{ selected.status === 'connected' ? $t('connectors.connected') : selected.status === 'error' ? $t('connectors.error') : $t('connectors.disconnected') }}
          </span>
        </div>
      </div>
    </template>

    <template #body>
      <MarketBrowser
        v-if="pageView === 'market'"
        kind="connector"
        :selected-key="marketSelectedKey"
        @installed="onMarketInstalled"
        @uninstalled="onMarketUninstalled"
        @view-installed="viewInstalledConnector"
      />
      <section v-else class="resource-section">
        <div class="resource-form-grid resource-form-grid--2">
          <label class="resource-field">
            <span class="resource-field__label">{{ $t('common.name') }}</span>
            <DqInput v-model="form.name" :placeholder="$t('connectors.nameExample')" />
          </label>
          <label class="resource-field">
            <span class="resource-field__label">{{ $t('connectors.transport') }}</span>
            <DqSelect v-model="form.transport" :placeholder="$t('connectors.transport')">
              <DqOption v-for="opt in transportOptions" :key="opt.value" :value="opt.value" :label="opt.label" />
            </DqSelect>
          </label>
        </div>
        <label class="resource-field resource-field--block">
          <span class="resource-field__label">{{ $t('common.description') }}</span>
          <DqInput v-model="form.description" type="textarea" :rows="3" :placeholder="$t('connectors.descriptionPlaceholder')" />
        </label>
        <p class="resource-field__hint">{{ $t('connectors.protocolHint') }}</p>
        <div v-if="form.transport === 'stdio'" class="resource-form-grid resource-form-grid--2">
          <label class="resource-field">
            <span class="resource-field__label">Command</span>
            <DqInput v-model="form.command" class="resource-input-mono" placeholder="npx" />
          </label>
          <label class="resource-field">
            <span class="resource-field__label">Args</span>
            <DqInput v-model="form.args" class="resource-input-mono" placeholder="-y @modelcontextprotocol/server-memory" />
          </label>
        </div>
        <div v-if="form.transport !== 'stdio'" class="resource-form-grid resource-form-grid--2">
          <label class="resource-field">
            <span class="resource-field__label">URL</span>
            <DqInput v-model="form.url" class="resource-input-mono" placeholder="http://localhost:3000/sse" />
          </label>
        </div>
        <label class="resource-field resource-field--block">
          <span class="resource-field__label">{{ $t('connectors.envVars') }}</span>
          <DqInput v-model="form.env" class="resource-input-mono" type="textarea" :rows="4" :placeholder="$t('connectors.envVarsPlaceholder')" />
        </label>
        <label v-if="form.transport !== 'stdio'" class="resource-field resource-field--block">
          <span class="resource-field__label">{{ $t('connectors.headers') }}</span>
          <DqInput v-model="headersText" class="resource-input-mono" type="textarea" :rows="3" :placeholder="$t('connectors.headersPlaceholder')" />
        </label>
        <label class="resource-field">
          <span class="resource-field__label">{{ $t('connectors.auth') }}</span>
          <DqSelect v-model="form.auth" :placeholder="$t('connectors.auth')">
            <DqOption v-for="opt in authOptions" :key="opt.value" :value="opt.value" :label="opt.label" />
          </DqSelect>
        </label>
        <label v-if="form.auth === 'headers' || form.auth === 'oauth'" class="resource-field resource-field--block">
          <span class="resource-field__label">{{ $t('connectors.headerSecrets') }}</span>
          <DqInput v-model="headerSecretsText" class="resource-input-mono" type="textarea" :rows="3" :placeholder="$t('connectors.headerSecretsPlaceholder')" />
        </label>
        <div v-if="!isCreating && form.auth === 'oauth'" class="resource-form-grid resource-form-grid--2">
          <label class="resource-field">
            <span class="resource-field__label">{{ $t('connectors.pasteAccessToken') }}</span>
            <DqInput v-model="accessToken" class="resource-input-mono" type="password" autocomplete="off" />
          </label>
          <div class="resource-field resource-field--toggle">
            <span class="resource-field__label">&nbsp;</span>
            <DqButton size="sm" @click="saveOAuthToken">{{ $t('connectors.completeOAuth') }}</DqButton>
          </div>
        </div>
        <!-- Discovered Tools -->
        <div class="resource-section__tools">
          <div class="resource-section__tools-header">
            <span class="resource-field__label">{{ $t('connectors.discoveredTools') }}</span>
            <DqButton size="sm" :disabled="refreshingTools" @click="handleRefreshTools">
              {{ refreshingTools ? $t('common.refreshing') : $t('connectors.refreshTools') }}
            </DqButton>
          </div>
          <div v-if="discoveredTools.length === 0" class="resource-section__tools-empty">
            {{ $t('connectors.noToolsDiscovered') }}
          </div>
          <div v-else class="resource-section__tools-list">
            <label v-for="tool in discoveredTools" :key="tool.name" class="resource-tool-row">
              <DqSwitch :model-value="tool.enabled" size="sm" @update:model-value="(v: boolean) => handleToggleTool(tool.name, v)" />
              <span class="resource-tool-row__name">{{ tool.name }}</span>
              <span v-if="tool.description" class="resource-tool-row__desc">{{ tool.description }}</span>
            </label>
          </div>
        </div>
        <div class="resource-form-grid resource-form-grid--2">
          <label v-if="!isCreating" class="resource-field resource-field--toggle">
            <span class="resource-field__label">{{ $t('connectors.enabled') }}</span>
            <DqSwitch
              :model-value="form.enabled"
              size="sm"
              @update:model-value="(v: boolean) => form.enabled = v"
            />
          </label>
          <label class="resource-field resource-field--toggle">
            <span class="resource-field__label">{{ $t('connectors.ambientMount') }}</span>
            <DqSwitch
              :model-value="form.ambientMount !== false"
              size="sm"
              @update:model-value="(v: boolean) => form.ambientMount = v"
            />
          </label>
        </div>
      </section>
    </template>

    <template v-if="pageView === 'library'" #footer>
      <span class="resource-workspace__hint">{{ $t('common.saveShortcut') }}</span>
      <div class="resource-workspace__footer-actions">
        <DqButton v-if="isCreating" @click="isCreating = false; selectedId = null">{{ $t('common.cancel') }}</DqButton>
        <DqButton v-if="!isCreating" @click="toggleEnabled">
          {{ selected?.enabled ? $t('connectors.disable') : $t('connectors.enable') }}
        </DqButton>
        <DqButton v-if="!isCreating" @click="removeSelected">{{ $t('common.delete') }}</DqButton>
        <DqButton type="primary" :disabled="saving" @click="save">
          {{ isCreating ? $t('connectors.createServer') : $t('common.save') }}
        </DqButton>
      </div>
    </template>
  </WorkspaceShell>
</template>

<style scoped>
.resource-rail__page-view {
  width: 100%;
}
.resource-rail__section > .resource-rail__section-head:first-child {
  padding-inline: 10px;
}
.resource-rail__section {
  display: flex;
  flex-direction: column;
  min-height: 0;
  flex: 1;
  overflow: hidden;
}
.resource-rail__section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 10px 6px 14px;
  flex-shrink: 0;
}
.resource-rail__section-title {
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--dq-label-tertiary);
}
.resource-rail__list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 0 6px 6px;
}
.resource-rail__empty {
  padding: 20px 12px;
}
</style>
