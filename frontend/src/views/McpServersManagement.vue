<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import WorkspaceShell from '@/components/common/WorkspaceShell.vue'
import { useMcpServersStore } from '@/stores/mcpServers'
import { confirm, toast } from '@/utils/feedback'
import type { MCPAuthMode, MCPServer, MCPToolDef } from '@/types'

type Transport = 'stdio' | 'sse' | 'streamable-http'

const { t } = useI18n()
const mcp = useMcpServersStore()

const selectedId = ref<string | null>(null)
const isCreating = ref(false)
const saving = ref(false)
const refreshingTools = ref(false)
const headerSecretsText = ref('')
const accessToken = ref('')
const installingCatalogId = ref<string | null>(null)

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
const hasSelection = computed(() => isCreating.value || !!selectedId.value)
const headerTitle = computed(() => {
  if (isCreating.value) return form.value.name.trim() || t('connectors.newServer')
  return selected.value?.name.trim() || t('connectors.untitled')
})

onMounted(async () => {
  // Soft-fail so a missing optional API (e.g. catalog on an old sidecar)
  // cannot take down the Connectors view via unhandledrejection.
  await Promise.allSettled([mcp.load(), mcp.loadCatalog()])
  if (sortedServers.value.length && !selectedId.value) {
    selectServer(sortedServers.value[0].id)
  }
})

function selectServer(id: string) {
  isCreating.value = false
  selectedId.value = id
  const server = mcp.items.find((s) => s.id === id)
  if (server) form.value = { ...server }
}

function openCreate() {
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

async function installFromCatalog(catalogId: string) {
  installingCatalogId.value = catalogId
  try {
    const server = await mcp.installCatalog(catalogId)
    toast.success(t('connectors.catalogInstalled'))
    selectServer(server.id)
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('common.saveFailed'))
  } finally {
    installingCatalogId.value = null
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
  if ((e.metaKey || e.ctrlKey) && e.key === 's') {
    e.preventDefault()
    save()
  }
}
</script>

<template>
  <WorkspaceShell
    :title="$t('connectors.title')"
    :count="sortedServers.length"
    :count-label="$t('connectors.countLabel')"
    :create-label="$t('connectors.newServer')"
    :has-selection="hasSelection"
    @create="openCreate"
    @keydown="onKeydown"
  >
    <template #rail>
      <div v-if="mcp.catalog.length" class="mcp-catalog">
        <div class="mcp-catalog__title">{{ $t('connectors.catalog') }}</div>
        <button
          v-for="entry in mcp.catalog"
          :key="entry.id"
          type="button"
          class="mcp-catalog__row"
          :disabled="installingCatalogId === entry.id"
          @click="installFromCatalog(entry.id)"
        >
          <span class="mcp-catalog__name">{{ entry.name }}</span>
          <span class="mcp-catalog__action">{{ $t('connectors.installCatalog') }}</span>
        </button>
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

    <template #empty>
      <DqEmpty :description="$t('connectors.emptySelection')">
        <p class="resource-workspace__hint">{{ $t('connectors.emptySelectionHint') }}</p>
      </DqEmpty>
    </template>

    <template #header>
      <div class="resource-workspace__identity">
        <h1 class="resource-workspace__title">{{ headerTitle }}</h1>
        <div v-if="!isCreating && selected" class="resource-workspace__badges">
          <span class="resource-status" :class="`resource-status--${selected.status}`">
            <span class="resource-status__dot" />
            {{ selected.status === 'connected' ? $t('connectors.connected') : selected.status === 'error' ? $t('connectors.error') : $t('connectors.disconnected') }}
          </span>
        </div>
      </div>
    </template>

    <template #body>
      <section class="resource-section">
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
        <div v-if="!isCreating" class="resource-form-grid resource-form-grid--2">
          <label class="resource-field resource-field--toggle">
            <span class="resource-field__label">{{ $t('connectors.enabled') }}</span>
            <DqSwitch
              :model-value="form.enabled"
              size="sm"
              @update:model-value="(v: boolean) => form.enabled = v"
            />
          </label>
        </div>
      </section>
    </template>

    <template #footer>
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
.mcp-catalog {
  padding: 8px 10px 4px;
  border-bottom: 1px solid var(--dq-border, rgba(0, 0, 0, 0.08));
  margin-bottom: 4px;
}
.mcp-catalog__title {
  font-size: 12px;
  opacity: 0.7;
  margin-bottom: 6px;
}
.mcp-catalog__row {
  display: flex;
  width: 100%;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 6px 8px;
  margin-bottom: 4px;
  border: 1px solid transparent;
  border-radius: 6px;
  background: transparent;
  cursor: pointer;
  text-align: left;
}
.mcp-catalog__row:hover {
  background: var(--dq-fill-muted, rgba(0, 0, 0, 0.04));
}
.mcp-catalog__name {
  font-size: 13px;
  flex: 1;
  min-width: 0;
}
.mcp-catalog__action {
  font-size: 12px;
  opacity: 0.75;
  white-space: nowrap;
}
</style>
