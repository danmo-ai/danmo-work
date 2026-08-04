<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import MdEditor from '@/components/common/MdEditor.vue'
import WorkspaceShell from '@/components/common/WorkspaceShell.vue'
import MarketBrowser from '@/components/market/MarketBrowser.vue'
import MarketCatalogRail from '@/components/market/MarketCatalogRail.vue'
import { useGlobalAgentsStore } from '@/stores/globalAgents'
import { useSkillsStore } from '@/stores/skills'
import { useKnowledgeStore } from '@/stores/knowledge'
import { useMarketStore } from '@/stores/market'
import { useRuntimeConfigStore } from '@/stores/runtimeConfig'
import { confirm, toast } from '@/utils/feedback'
import type { Agent, ToolBinding } from '@/types'

type ConfigTab = 'overview' | 'prompt' | 'skills' | 'tools' | 'knowledge'
type PageView = 'library' | 'market'

type AgentForm = Agent

const { t } = useI18n()
const globalAgents = useGlobalAgentsStore()
const skills = useSkillsStore()
const knowledge = useKnowledgeStore()
const marketStore = useMarketStore()
const runtimeConfig = useRuntimeConfigStore()

const globalMaxSteps = computed(() => runtimeConfig.config?.maxStepsDefault ?? 200)
const stepsLabel = computed(() => {
  const n = agentForm.value.steps
  if (!n || n <= 0) return t('teams.maxStepsFollowGlobal', { n: globalMaxSteps.value })
  return String(n)
})

const pageView = ref<PageView>('library')
const pageViewOptions = computed(() => [
  { label: t('market.library'), value: 'library' as const },
  { label: t('market.tab'), value: 'market' as const },
])
const selectedId = ref<string | null>(null)
const marketSelectedKey = ref<string | null>(null)
const isCreating = ref(false)
const saving = ref(false)
const activeTab = ref<ConfigTab>('overview')
const pendingTool = ref<ToolBinding>({ toolId: '', riskLevel: 'low' })
const pendingMcpServer = ref('')

function defaultInheritAmbient(mode?: Agent['mode']): boolean {
  return mode !== 'subagent'
}

function emptyAgentForm(): AgentForm {
  return {
    id: '',
    name: '',
    description: '',
    persona: '',
    mode: 'primary',
    systemPrompt: '',
    skillIds: [],
    tools: [],
    mcpServers: [],
    knowledgeIds: [],
    steps: 0,
    canDelegate: false,
    inheritAmbient: true,
  }
}

const agentForm = ref<AgentForm>(emptyAgentForm())

const sortedAgents = computed(() =>
  [...globalAgents.items].sort((a, b) => a.name.localeCompare(b.name, 'zh-CN')),
)

const primaryAgents = computed(() =>
  sortedAgents.value.filter((a) => a.mode !== 'subagent'),
)

const builtinAgents = computed(() =>
  sortedAgents.value.filter((a) => a.mode === 'subagent' && a.builtin && !a.marketSource),
)

const customAgents = computed(() =>
  sortedAgents.value.filter((a) => a.mode === 'subagent' && !a.builtin && !a.marketSource),
)

const marketAgents = computed(() =>
  sortedAgents.value.filter((a) => a.mode === 'subagent' && !!a.marketSource),
)

/** Experts that a lead can summon when canDelegate is on (read-only roster). */
const collaboratableExperts = computed(() =>
  sortedAgents.value.filter((a) => a.mode === 'subagent'),
)

const capabilitySummary = computed(() => ({
  skills: agentForm.value.skillIds?.length ?? 0,
  tools: agentForm.value.tools?.length ?? 0,
  connectors: agentForm.value.mcpServers?.length ?? 0,
  knowledge: agentForm.value.knowledgeIds?.length ?? 0,
  canDelegate: !!agentForm.value.canDelegate,
}))

const selectedAgent = computed(() => globalAgents.items.find((a) => a.id === selectedId.value))
const marketSelected = computed(() => {
  if (!marketSelectedKey.value) return null
  return (
    marketStore.catalog.find(
      (item) => item.kind === 'expert' && `${item.sourceId}:${item.id}` === marketSelectedKey.value,
    ) ?? null
  )
})
const hasSelection = computed(
  () =>
    (pageView.value === 'market' && !!marketSelectedKey.value) ||
    isCreating.value ||
    !!selectedId.value,
)

const hasCodegraphExpert = computed(() =>
  sortedAgents.value.some((a) => a.id === 'codegraph'),
)

async function openCodegraphMarket() {
  await Promise.all([marketStore.loadSources(), marketStore.loadCatalog()])
  const item =
    marketStore.catalog.find((i) => i.kind === 'expert' && i.id === 'codegraph' && !i.installed) ??
    marketStore.catalog.find((i) => i.kind === 'expert' && i.id === 'codegraph')
  if (item) {
    marketSelectedKey.value = `${item.sourceId}:${item.id}`
  }
  pageView.value = 'market'
}

const headerTitle = computed(() => {
  if (pageView.value === 'market') {
    return marketSelected.value?.name || t('market.tab')
  }
  if (isCreating.value) return agentForm.value.name.trim() || t('teams.newAgent')
  return selectedAgent.value?.name.trim() || t('teams.untitled')
})

async function onMarketInstalled(id: string) {
  await Promise.all([globalAgents.load(), skills.load()])
  // Stay on market so deps script logs + connector jump links remain visible.
  void id
}

function viewInstalledExpert(id: string) {
  if (!id) return
  pageView.value = 'library'
  if (globalAgents.items.some((a) => a.id === id)) {
    selectAgent(id)
  }
}

async function onMarketUninstalled() {
  await Promise.all([globalAgents.load(), skills.load()])
  if (selectedId.value && !globalAgents.items.some((a) => a.id === selectedId.value)) {
    selectedId.value = null
  }
}

const sectionTabs = computed(() => [
  { id: 'overview' as const, label: t('teams.overview') },
  { id: 'prompt' as const, label: t('teams.prompt') },
  { id: 'skills' as const, label: t('teams.skills') },
  { id: 'tools' as const, label: t('teams.tools') },
  { id: 'knowledge' as const, label: t('teams.knowledge') },
])

onMounted(async () => {
  await Promise.all([globalAgents.load(), skills.load(), knowledge.loadBases(), runtimeConfig.loadConfig()])
  if (sortedAgents.value.length && !selectedId.value) {
    selectAgent(sortedAgents.value[0].id)
  }
})

function selectAgent(id: string) {
  isCreating.value = false
  selectedId.value = id
  activeTab.value = 'overview'
  const agent = globalAgents.items.find((a) => a.id === id)
  if (agent) {
    agentForm.value = {
      ...agent,
      skillIds: agent.skillIds ? [...agent.skillIds] : [],
      tools: agent.tools ? [...agent.tools] : [],
      mcpServers: agent.mcpServers ? [...agent.mcpServers] : [],
      knowledgeIds: agent.knowledgeIds ? [...agent.knowledgeIds] : [],
      inheritAmbient: agent.inheritAmbient ?? defaultInheritAmbient(agent.mode),
    }
  }
}

function openCreate() {
  isCreating.value = true
  selectedId.value = null
  activeTab.value = 'overview'
  agentForm.value = emptyAgentForm()
}

const ID_RE = /^[a-zA-Z][a-zA-Z0-9_-]{1,63}$/

async function save() {
  if (!agentForm.value.id || !ID_RE.test(agentForm.value.id)) {
    toast.warning(t('teams.idRule'))
    activeTab.value = 'overview'
    return
  }
  if (!agentForm.value.name) {
    toast.warning(t('teams.namePlaceholder'))
    activeTab.value = 'overview'
    return
  }

  saving.value = true
  try {
    if (isCreating.value) {
      await globalAgents.create({
        id: agentForm.value.id,
        name: agentForm.value.name,
        description: agentForm.value.description,
        persona: agentForm.value.persona,
        mode: agentForm.value.mode ?? 'primary',
        systemPrompt: agentForm.value.systemPrompt,
        steps: agentForm.value.steps ?? 0,
        skillIds: agentForm.value.skillIds,
        tools: agentForm.value.tools,
        mcpServers: agentForm.value.mcpServers,
        knowledgeIds: agentForm.value.knowledgeIds,
        canDelegate: agentForm.value.canDelegate ?? false,
        inheritAmbient: agentForm.value.inheritAmbient ?? defaultInheritAmbient(agentForm.value.mode),
      })
      toast.success(t('teams.created'))
      isCreating.value = false
      selectAgent(agentForm.value.id)
    } else if (selectedAgent.value) {
      await globalAgents.update(selectedAgent.value.id, {
        name: agentForm.value.name,
        description: agentForm.value.description,
        persona: agentForm.value.persona,
        mode: agentForm.value.mode ?? 'primary',
        systemPrompt: agentForm.value.systemPrompt,
        steps: agentForm.value.steps ?? 0,
        skillIds: agentForm.value.skillIds,
        tools: agentForm.value.tools,
        mcpServers: agentForm.value.mcpServers,
        knowledgeIds: agentForm.value.knowledgeIds,
        canDelegate: agentForm.value.canDelegate ?? false,
        inheritAmbient: agentForm.value.inheritAmbient ?? defaultInheritAmbient(agentForm.value.mode),
      })

      toast.success(t('teams.saved'))
      selectAgent(selectedAgent.value.id)
    }
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('common.saveFailed'))
  } finally {
    saving.value = false
  }
}

async function removeSelected() {
  if (!selectedAgent.value) return
  try {
    await confirm(t('teams.deleteConfirm', { name: selectedAgent.value.name }), t('teams.deleteAgent'), { type: 'warning' })
    await globalAgents.remove(selectedAgent.value.id)
    selectedId.value = null
    isCreating.value = false
    toast.success(t('teams.deleted'))
  } catch (e) {
    if (e instanceof Error) toast.error(e.message)
  }
}

async function resetSelected() {
  if (!selectedAgent.value) return
  try {
    await confirm(t('teams.resetConfirm', { name: selectedAgent.value.name }), t('teams.resetAgent'), { type: 'warning' })
  } catch {
    return
  }
  try {
    const a = await globalAgents.reset(selectedAgent.value.id)
    selectAgent(a.id)
    toast.success(t('teams.reset'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('teams.resetFailed'))
  }
}

function addTool() {
  const toolId = pendingTool.value.toolId?.trim() ?? ''
  if (!toolId) return
  agentForm.value.tools = [
    ...(agentForm.value.tools ?? []),
    { ...pendingTool.value, toolId },
  ]
  pendingTool.value = { toolId: '', riskLevel: 'low' }
}

function removeTool(idx: number) {
  agentForm.value.tools = (agentForm.value.tools ?? []).filter((_, i) => i !== idx)
}

function addMcpServer() {
  const id = pendingMcpServer.value.trim()
  if (!id || id === '*') return
  const cur = agentForm.value.mcpServers ?? []
  if (cur.includes(id)) {
    pendingMcpServer.value = ''
    return
  }
  agentForm.value.mcpServers = [...cur, id]
  pendingMcpServer.value = ''
}

function removeMcpServer(idx: number) {
  agentForm.value.mcpServers = (agentForm.value.mcpServers ?? []).filter((_, i) => i !== idx)
}

function toggleSkill(id: string) {
  const ids = agentForm.value.skillIds ?? []
  agentForm.value.skillIds = ids.includes(id) ? ids.filter((x) => x !== id) : [...ids, id]
}

function toggleKnowledge(id: string) {
  const ids = agentForm.value.knowledgeIds ?? []
  agentForm.value.knowledgeIds = ids.includes(id) ? ids.filter((x) => x !== id) : [...ids, id]
}

function agentInitial(name: string) {
  return name.trim().charAt(0).toUpperCase() || '?'
}

function compactId(id: string) {
  if (id.length <= 20) return id
  return `${id.slice(0, 8)}…${id.slice(-4)}`
}

/** Rail subtitle: role first, id only as fallback. */
function agentRailSubtitle(agent: Agent): string {
  const text = (agent.persona || agent.description || '').trim().replace(/\s+/g, ' ')
  if (text) return text.length > 56 ? `${text.slice(0, 56)}…` : text
  return compactId(agent.id)
}

function expertOneLiner(agent: Agent): string {
  const text = (agent.persona || agent.description || '').trim().replace(/\s+/g, ' ')
  if (!text) return compactId(agent.id)
  return text.length > 72 ? `${text.slice(0, 72)}…` : text
}

function onWorkspaceKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === 's') {
    e.preventDefault()
    void save()
  }
}
</script>

<template>
  <WorkspaceShell
    custom-rail
    :has-selection="hasSelection"
    @keydown="onWorkspaceKeydown"
    @create="openCreate"
  >
    <template #rail>
      <div class="resource-rail__section">
        <div class="resource-rail__section-head">
          <DqSegmented v-model="pageView" block class="resource-rail__page-view" :options="pageViewOptions" />
        </div>
        <template v-if="pageView === 'library'">
        <div class="resource-rail__section-head">
          <span class="resource-rail__section-title">{{ $t('teams.workerAgent') }}</span>
          <DqIconButton :aria-label="$t('teams.newAgent')" @click="openCreate">
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M12 5v14M5 12h14" stroke-linecap="round" />
            </svg>
          </DqIconButton>
        </div>
        <DqEmpty v-if="!sortedAgents.length" class="resource-rail__empty" :description="$t('teams.noWorkers')" />
        <template v-else>
          <div v-if="primaryAgents.length" class="resource-rail__group">
            <div class="resource-rail__group-title">{{ $t('teams.primaryAgents') }}</div>
            <nav class="resource-rail__list" :aria-label="$t('teams.primaryAgents')">
              <button
                v-for="agent in primaryAgents"
                :key="agent.id"
                type="button"
                class="resource-rail__row"
                :class="{ 'is-active': selectedId === agent.id && !isCreating }"
                @click="selectAgent(agent.id)"
              >
                <span class="resource-rail__avatar">{{ agentInitial(agent.name) }}</span>
                <span class="resource-rail__meta">
                  <span class="resource-rail__name">{{ agent.name }}</span>
                  <span class="resource-rail__desc" :title="agent.persona || agent.description || agent.id">{{ agentRailSubtitle(agent) }}</span>
                </span>
              </button>
            </nav>
          </div>
          <div v-if="builtinAgents.length" class="resource-rail__group">
            <div class="resource-rail__group-title">{{ $t('teams.builtinAgents') }}</div>
            <nav class="resource-rail__list" :aria-label="$t('teams.builtinAgents')">
              <button
                v-for="agent in builtinAgents"
                :key="agent.id"
                type="button"
                class="resource-rail__row"
                :class="{ 'is-active': selectedId === agent.id && !isCreating }"
                @click="selectAgent(agent.id)"
              >
                <span class="resource-rail__avatar">{{ agentInitial(agent.name) }}</span>
                <span class="resource-rail__meta">
                  <span class="resource-rail__name">{{ agent.name }}</span>
                  <span class="resource-rail__desc" :title="agent.persona || agent.description || agent.id">{{ agentRailSubtitle(agent) }}</span>
                </span>
              </button>
            </nav>
          </div>
          <div v-if="customAgents.length" class="resource-rail__group">
            <div class="resource-rail__group-title">{{ $t('teams.customAgents') }}</div>
            <nav class="resource-rail__list" :aria-label="$t('teams.customAgents')">
              <button
                v-for="agent in customAgents"
                :key="agent.id"
                type="button"
                class="resource-rail__row"
                :class="{ 'is-active': selectedId === agent.id && !isCreating }"
                @click="selectAgent(agent.id)"
              >
                <span class="resource-rail__avatar">{{ agentInitial(agent.name) }}</span>
                <span class="resource-rail__meta">
                  <span class="resource-rail__name">{{ agent.name }}</span>
                  <span class="resource-rail__desc" :title="agent.persona || agent.description || agent.id">{{ agentRailSubtitle(agent) }}</span>
                </span>
              </button>
            </nav>
          </div>
          <div v-if="marketAgents.length" class="resource-rail__group">
            <div class="resource-rail__group-title">{{ $t('teams.marketAgents') }}</div>
            <nav class="resource-rail__list" :aria-label="$t('teams.marketAgents')">
              <button
                v-for="agent in marketAgents"
                :key="agent.id"
                type="button"
                class="resource-rail__row"
                :class="{ 'is-active': selectedId === agent.id && !isCreating }"
                @click="selectAgent(agent.id)"
              >
                <span class="resource-rail__avatar">{{ agentInitial(agent.name) }}</span>
                <span class="resource-rail__meta">
                  <span class="resource-rail__name">{{ agent.name }}</span>
                  <span class="resource-rail__desc" :title="agent.persona || agent.description || agent.id">{{ agentRailSubtitle(agent) }}</span>
                </span>
              </button>
            </nav>
          </div>
          <div v-if="!hasCodegraphExpert" class="resource-rail__group">
            <p class="resource-workspace__hint">{{ $t('teams.codegraphMarketHint') }}</p>
            <DqButton size="sm" @click="openCodegraphMarket">{{ $t('teams.installCodegraph') }}</DqButton>
          </div>
        </template>
        </template>
        <MarketCatalogRail v-else v-model:selected-key="marketSelectedKey" kind="expert" />
      </div>
    </template>

    <template #empty>
      <DqEmpty :description="$t('teams.emptySelection')">
        <p class="resource-workspace__hint">{{ $t('teams.emptySelectionHint') }}</p>
        <div class="resource-workspace__hint-actions">
          <DqButton @click="pageView = 'market'">{{ $t('market.tab') }}</DqButton>
          <DqButton v-if="!hasCodegraphExpert" @click="openCodegraphMarket">
            {{ $t('teams.installCodegraph') }}
          </DqButton>
        </div>
      </DqEmpty>
    </template>

    <template #header>
      <div class="resource-workspace__identity">
        <h1 class="resource-workspace__title">{{ headerTitle }}</h1>
        <div v-if="pageView === 'library' && !isCreating" class="resource-workspace__badges">
          <code v-if="selectedAgent?.id" class="resource-workspace__id">
            {{ compactId(selectedAgent.id) }}
          </code>
        </div>
      </div>
      <DqSegmented
        v-if="pageView === 'library'"
        v-model="activeTab"
        class="resource-workspace__segmented"
        :options="sectionTabs.map((t) => ({ label: t.label, value: t.id }))"
      />
    </template>

    <template #body>
      <MarketBrowser
        v-if="pageView === 'market'"
        kind="expert"
        :selected-key="marketSelectedKey"
        @installed="onMarketInstalled"
        @uninstalled="onMarketUninstalled"
        @view-installed="viewInstalledExpert"
      />
      <template v-else>
      <section v-show="activeTab === 'overview'" class="resource-section">
        <div class="expert-capability" aria-label="capability summary">
          <span class="expert-capability__label">{{ $t('teams.capabilitySummary') }}</span>
          <div class="expert-capability__chips">
            <span class="expert-capability__chip">{{ $t('teams.capabilitySkills', { n: capabilitySummary.skills }) }}</span>
            <span class="expert-capability__chip">{{ $t('teams.capabilityTools', { n: capabilitySummary.tools }) }}</span>
            <span class="expert-capability__chip">{{ $t('teams.capabilityConnectors', { n: capabilitySummary.connectors }) }}</span>
            <span class="expert-capability__chip">{{ $t('teams.capabilityKnowledge', { n: capabilitySummary.knowledge }) }}</span>
            <span
              class="expert-capability__chip"
              :class="capabilitySummary.canDelegate ? 'is-on' : 'is-off'"
            >
              {{ capabilitySummary.canDelegate ? $t('teams.capabilityCanDelegate') : $t('teams.capabilityNoDelegate') }}
            </span>
          </div>
        </div>
        <div class="resource-form-grid resource-form-grid--2">
          <label class="resource-field">
            <span class="resource-field__label">{{ $t('teams.agentId') }}</span>
            <DqInput v-model="agentForm.id" class="resource-input-mono" placeholder="alert-analyst" :disabled="!isCreating" />
            <span v-if="isCreating" class="resource-field__hint">{{ $t('teams.idHint') }}</span>
          </label>
          <label class="resource-field">
            <span class="resource-field__label">{{ $t('teams.displayName') }}</span>
            <DqInput v-model="agentForm.name" placeholder="Alert Analyst" />
          </label>
        </div>
        <div class="resource-form-grid resource-form-grid--2">
          <label class="resource-field">
            <span class="resource-field__label">{{ $t('teams.mode') }}</span>
            <DqSelect v-model="agentForm.mode" :placeholder="$t('teams.mode')">
              <DqOption value="primary" :label="$t('teams.primary')" />
              <DqOption value="subagent" :label="$t('teams.subagent')" />
            </DqSelect>
          </label>
          <div class="resource-field">
            <span class="resource-field__label">{{ $t('teams.maxSteps') }}</span>
            <div class="slider-row">
              <DqSlider v-model="agentForm.steps" :min="0" :max="500" :step="1" />
              <span class="slider-row__value">{{ stepsLabel }}</span>
            </div>
            <span class="resource-field__hint">{{ $t('teams.maxStepsHint') }}</span>
          </div>
        </div>
        <label class="resource-field resource-field--block">
          <span class="resource-field__label">{{ $t('teams.persona') }}</span>
          <DqInput v-model="agentForm.persona" :placeholder="$t('teams.personaPlaceholder')" />
        </label>
        <label class="resource-field resource-field--block">
          <span class="resource-field__label">{{ $t('common.description') }}</span>
          <DqInput v-model="agentForm.description" type="textarea" :rows="4" :placeholder="$t('teams.descriptionPlaceholder')" />
        </label>
        <div class="resource-field resource-field--block resource-field--inline" @click="agentForm.canDelegate = !agentForm.canDelegate">
          <div class="resource-field__inline-meta">
            <span class="resource-field__label">{{ $t('teams.canDelegate') }}</span>
            <span class="resource-field__hint">{{ $t('teams.canDelegateHint') }}</span>
          </div>
          <DqSwitch :model-value="agentForm.canDelegate" size="sm" />
        </div>
        <div
          v-if="agentForm.canDelegate && agentForm.mode !== 'subagent'"
          class="expert-collab"
        >
          <div class="expert-collab__head">
            <span class="resource-field__label">{{ $t('teams.collaboratableExperts') }}</span>
            <span class="resource-field__hint">{{ $t('teams.collaboratableExpertsHint') }}</span>
          </div>
          <DqEmpty
            v-if="!collaboratableExperts.length"
            :description="$t('teams.noCollaboratableExperts')"
          />
          <ul v-else class="expert-collab__list">
            <li v-for="expert in collaboratableExperts" :key="expert.id" class="expert-collab__item">
              <span class="expert-collab__name">{{ expert.name }}</span>
              <span class="expert-collab__desc">{{ expertOneLiner(expert) }}</span>
            </li>
          </ul>
        </div>
        <div class="resource-field resource-field--block resource-field--inline" @click="agentForm.inheritAmbient = !agentForm.inheritAmbient">
          <div class="resource-field__inline-meta">
            <span class="resource-field__label">{{ $t('teams.inheritAmbient') }}</span>
            <span class="resource-field__hint">{{ $t('teams.inheritAmbientHint') }}</span>
          </div>
          <DqSwitch :model-value="agentForm.inheritAmbient" size="sm" />
        </div>
      </section>

      <section v-show="activeTab === 'prompt'" class="resource-section resource-section--prompt">
        <MdEditor v-model="agentForm.systemPrompt" :rows="18" :placeholder="$t('teams.promptPlaceholder')" />
      </section>

      <section v-show="activeTab === 'skills'" class="resource-section">
        <DqEmpty v-if="!skills.items.length" :description="$t('teams.noSkills')" />
        <div v-else class="resource-list-card">
          <div
            v-for="skill in skills.items"
            :key="skill.id"
            class="resource-list-card__item"
            :class="{ 'is-active': agentForm.skillIds?.includes(skill.id) }"
            @click="toggleSkill(skill.id)"
          >
            <DqCheckbox :model-value="agentForm.skillIds?.includes(skill.id)" />
            <div class="resource-list-card__meta">
              <span class="resource-list-card__name">{{ skill.name }}</span>
              <span class="resource-list-card__desc">{{ skill.description }}</span>
            </div>
          </div>
        </div>
      </section>

      <section v-show="activeTab === 'tools'" class="resource-section resource-section--tools">
        <div class="resource-panel">
          <div class="resource-section__head">
            <h3 class="resource-section__title">{{ $t('teams.builtinToolsLabel') }}</h3>
            <p class="resource-section__desc">{{ $t('teams.builtinToolsHint') }}</p>
          </div>
          <div class="resource-form-grid resource-form-grid--3">
            <label class="resource-field">
              <span class="resource-field__label">{{ $t('toolsPage.toolId') }}</span>
              <DqInput
                v-model="pendingTool.toolId"
                class="resource-input-mono"
                :placeholder="$t('teams.toolIdPlaceholder')"
                @keydown.enter.prevent="addTool"
              />
            </label>
            <label class="resource-field">
              <span class="resource-field__label">{{ $t('common.riskLevel') }}</span>
              <DqSelect v-model="pendingTool.riskLevel" :placeholder="$t('common.riskLevel')">
                <DqOption value="low" :label="$t('common.riskLow')" />
                <DqOption value="medium" :label="$t('common.riskMedium')" />
                <DqOption value="high" :label="$t('common.riskHigh')" />
                <DqOption value="external" :label="$t('common.riskExternal')" />
              </DqSelect>
            </label>
            <div class="resource-field resource-field--action">
              <DqButton type="primary" @click="addTool">{{ $t('common.addTool') }}</DqButton>
            </div>
          </div>
          <DqEmpty v-if="!(agentForm.tools?.length)" :description="$t('teams.noTools')" />
          <div v-else class="resource-list-card">
            <div v-for="(tool, idx) in agentForm.tools" :key="`${tool.toolId}-${idx}`" class="resource-list-card__item">
              <div class="resource-list-card__meta">
                <span class="resource-list-card__name resource-list-card__name--mono">{{ tool.toolId }}</span>
              </div>
              <div class="resource-list-card__actions">
                <DqSelect v-model="tool.riskLevel" class="resource-list-card__risk-select" size="sm">
                  <DqOption value="low" :label="$t('common.riskLow')" />
                  <DqOption value="medium" :label="$t('common.riskMedium')" />
                  <DqOption value="high" :label="$t('common.riskHigh')" />
                  <DqOption value="external" :label="$t('common.riskExternal')" />
                </DqSelect>
                <button type="button" class="resource-list-card__action resource-list-card__action--danger" @click="removeTool(idx)">{{ $t('common.delete') }}</button>
              </div>
            </div>
          </div>
        </div>

        <div class="resource-panel">
          <div class="resource-section__head">
            <h3 class="resource-section__title">{{ $t('teams.mcpServersLabel') }}</h3>
            <p class="resource-section__desc">{{ $t('teams.mcpServerHint') }}</p>
          </div>
          <div class="resource-add-row">
            <label class="resource-field">
              <span class="resource-field__label">{{ $t('teams.connectorIdLabel') }}</span>
              <DqInput
                v-model="pendingMcpServer"
                class="resource-input-mono"
                :placeholder="$t('teams.mcpServerPlaceholder')"
                @keydown.enter.prevent="addMcpServer"
              />
            </label>
            <div class="resource-field resource-field--action">
              <DqButton type="primary" @click="addMcpServer">{{ $t('teams.addConnector') }}</DqButton>
            </div>
          </div>
          <DqEmpty v-if="!(agentForm.mcpServers?.length)" :description="$t('teams.noMcpServers')" />
          <div v-else class="resource-list-card">
            <div v-for="(sid, idx) in agentForm.mcpServers" :key="sid" class="resource-list-card__item">
              <div class="resource-list-card__meta">
                <span class="resource-list-card__name resource-list-card__name--mono">{{ sid }}</span>
              </div>
              <div class="resource-list-card__actions">
                <button type="button" class="resource-list-card__action resource-list-card__action--danger" @click="removeMcpServer(idx)">{{ $t('common.delete') }}</button>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section v-show="activeTab === 'knowledge'" class="resource-section">
        <DqEmpty v-if="!knowledge.bases.length" :description="$t('teams.noKnowledge')" />
        <div v-else class="resource-list-card">
          <div
            v-for="base in knowledge.bases"
            :key="base.id"
            class="resource-list-card__item"
            @click="toggleKnowledge(base.id)"
          >
            <DqCheckbox :model-value="agentForm.knowledgeIds?.includes(base.id)" />
            <div class="resource-list-card__meta">
              <span class="resource-list-card__name">{{ base.name }}</span>
              <span class="resource-list-card__desc">{{ $t('teams.documents', { count: base.documentCount }) }}</span>
            </div>
          </div>
        </div>
      </section>
      </template>
    </template>

    <template #footer>
      <template v-if="pageView === 'library'">
        <span class="resource-workspace__hint">{{ $t('common.saveShortcut') }}</span>
        <div class="resource-workspace__footer-actions">
          <DqButton v-if="isCreating" @click="isCreating = false; selectedId = null">{{ $t('common.cancel') }}</DqButton>
          <DqButton v-if="!isCreating" @click="removeSelected">{{ $t('common.delete') }}</DqButton>
          <DqButton v-if="!isCreating" @click="resetSelected">{{ $t('common.reset') }}</DqButton>
          <DqButton type="primary" :disabled="saving" @click="save">
            {{ isCreating ? $t('teams.createAgent') : $t('common.save') }}
          </DqButton>
        </div>
      </template>
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

.resource-rail__group + .resource-rail__group {
  margin-top: 8px;
}

.resource-rail__group-title {
  padding: 8px 12px 4px;
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--dq-label-tertiary);
}

.resource-section--prompt {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}

.resource-section--prompt .work-md {
  flex: 1;
  min-height: 360px;
}

.resource-list-card__risk-select {
  width: 96px;
}

.resource-list-card__actions .resource-list-card__risk-select {
  margin-right: 4px;
}

.slider-row {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 36px;
}

.slider-row :deep(.dq-slider) {
  flex: 1;
  min-width: 0;
}

.slider-row__value {
  flex-shrink: 0;
  min-width: 36px;
  text-align: right;
  font-size: var(--dq-font-size-footnote);
  font-weight: 500;
  font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, monospace;
  color: var(--dq-label-secondary);
}

.expert-capability {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 16px;
  padding: 12px 14px;
  border-radius: 10px;
  border: 1px solid color-mix(in srgb, var(--dq-label-primary) 8%, transparent);
  background: color-mix(in srgb, var(--dq-label-primary) 2.5%, transparent);
}

.expert-capability__label {
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--dq-label-tertiary);
}

.expert-capability__chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.expert-capability__chip {
  display: inline-flex;
  align-items: center;
  padding: 3px 8px;
  border-radius: 6px;
  font-size: var(--dq-font-size-caption);
  font-weight: 500;
  color: var(--dq-label-secondary);
  background: color-mix(in srgb, var(--dq-label-primary) 6%, transparent);
}

.expert-capability__chip.is-on {
  color: var(--dq-accent);
  background: color-mix(in srgb, var(--dq-accent) 12%, transparent);
}

.expert-capability__chip.is-off {
  color: var(--dq-label-tertiary);
}

.expert-collab {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 4px 0 12px;
  padding: 12px 14px;
  border-radius: 10px;
  border: 1px solid color-mix(in srgb, var(--dq-label-primary) 8%, transparent);
}

.expert-collab__head {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.expert-collab__list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.expert-collab__item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 8px 10px;
  border-radius: 8px;
  background: color-mix(in srgb, var(--dq-label-primary) 3%, transparent);
}

.expert-collab__name {
  font-size: var(--dq-font-size-footnote);
  font-weight: 600;
  color: var(--dq-label-primary);
}

.expert-collab__desc {
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-secondary);
  line-height: 1.35;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
