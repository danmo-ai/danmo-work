<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import WorkspaceShell from '@/components/common/WorkspaceShell.vue'
import { useAutomationsStore } from '@/stores/automations'
import { useGlobalAgentsStore } from '@/stores/globalAgents'
import { confirm, toast } from '@/utils/feedback'
import type { Automation, AutomationTrigger } from '@/types'

const { t } = useI18n()
const automations = useAutomationsStore()
const agents = useGlobalAgentsStore()

const selectedId = ref<string | null>(null)
const isCreating = ref(false)
const saving = ref(false)
const listFilter = ref<'current' | 'paused' | 'all'>('current')

const triggerOptions = computed<{ value: AutomationTrigger; label: string }[]>(() => [
  { value: 'schedule', label: t('automations.schedule') },
  { value: 'event', label: t('automations.event') },
  { value: 'webhook', label: t('automations.webhook') },
  { value: 'manual', label: t('automations.manual') },
])

const form = ref<Automation>({
  id: '',
  name: '',
  description: '',
  enabled: true,
  trigger: 'manual',
  schedule: '',
  eventType: '',
  webhookPath: '',
  agentId: '',
  prompt: '',
})

const sortedAutomations = computed(() => {
  let list = [...automations.items]
  if (listFilter.value === 'current') list = list.filter((a) => a.enabled)
  else if (listFilter.value === 'paused') list = list.filter((a) => !a.enabled)
  return list.sort((a, b) => a.name.localeCompare(b.name, 'zh-CN'))
})
const selected = computed(() => automations.items.find((a) => a.id === selectedId.value))
const hasSelection = computed(() => isCreating.value || !!selectedId.value)
const headerTitle = computed(() => {
  if (isCreating.value) return form.value.name.trim() || t('automations.newAutomation')
  return selected.value?.name.trim() || t('automations.untitled')
})

onMounted(async () => {
  await Promise.allSettled([agents.load(), automations.load()])
  if (sortedAutomations.value.length && !selectedId.value) {
    selectAutomation(sortedAutomations.value[0].id)
  }
})

function selectAutomation(id: string) {
  isCreating.value = false
  selectedId.value = id
  const item = automations.items.find((a) => a.id === id)
  if (item) form.value = { ...item }
}

function openCreate() {
  isCreating.value = true
  selectedId.value = null
  form.value = {
    id: '',
    name: '',
    description: '',
    enabled: true,
    trigger: 'manual',
    schedule: '',
    eventType: '',
    webhookPath: '',
    agentId: '',
    prompt: '',
  }
}

async function save() {
  if (!form.value.name.trim()) {
    toast.warning(t('automations.namePlaceholder'))
    return
  }
  if (form.value.trigger === 'schedule' && !form.value.schedule?.trim()) {
    toast.warning(t('automations.schedulePlaceholder'))
    return
  }
  if (form.value.trigger === 'event' && !form.value.eventType?.trim()) {
    toast.warning(t('automations.eventPlaceholder'))
    return
  }
  saving.value = true
  try {
    if (isCreating.value) {
      const item = await automations.create({ ...form.value, name: form.value.name.trim() })
      toast.success(t('automations.created'))
      isCreating.value = false
      selectAutomation(item.id)
    } else if (selected.value) {
      await automations.update(selected.value.id, { ...form.value })
      toast.success(t('automations.saved'))
      selectAutomation(selected.value.id)
    }
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('common.saveFailed'))
  } finally {
    saving.value = false
  }
}

async function runSelected() {
  if (!selected.value) return
  try {
    const a = await automations.run(selected.value.id)
    toast.success(t('automations.runStarted'))
    selectAutomation(a.id)
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('automations.runFailed'))
  }
}

async function removeSelected() {
  if (!selected.value) return
  try {
    await confirm(t('automations.deleteConfirm', { name: selected.value.name }), t('automations.deleteTitle'), { type: 'warning' })
  } catch {
    return
  }
  await automations.remove(selected.value.id)
  selectedId.value = null
  isCreating.value = false
  toast.success(t('automations.deleted'))
}

async function toggleEnabled() {
  if (!selected.value) return
  const a = await automations.toggle(selected.value.id)
  selectAutomation(a.id)
}

function initial(name: string) {
  return name.trim().charAt(0).toUpperCase() || 'A'
}

function triggerLabel(value: AutomationTrigger) {
  return triggerOptions.value.find((o) => o.value === value)?.label ?? value
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
    :title="$t('automations.title')"
    :count="automations.items.length"
    :count-label="$t('automations.title')"
    :create-label="$t('automations.newAutomation')"
    :has-selection="hasSelection"
    @create="openCreate"
    @keydown="onKeydown"
  >
    <template #rail>
      <div class="automations-filter">
        <button type="button" :class="{ 'is-active': listFilter === 'current' }" @click="listFilter = 'current'">{{ $t('automations.filterCurrent') }}</button>
        <button type="button" :class="{ 'is-active': listFilter === 'paused' }" @click="listFilter = 'paused'">{{ $t('automations.filterPaused') }}</button>
        <button type="button" :class="{ 'is-active': listFilter === 'all' }" @click="listFilter = 'all'">{{ $t('automations.filterAll') }}</button>
      </div>
      <DqEmpty v-if="!sortedAutomations.length" class="resource-rail__empty" :description="$t('automations.noAutomations')" />
      <nav v-else class="resource-rail__list" :aria-label="$t('automations.automationList')">
        <button
          v-for="item in sortedAutomations"
          :key="item.id"
          type="button"
          class="resource-rail__row"
          :class="{ 'is-active': selectedId === item.id && !isCreating }"
          @click="selectAutomation(item.id)"
        >
          <span class="resource-rail__avatar">{{ initial(item.name) }}</span>
          <span class="resource-rail__meta">
            <span class="resource-rail__name">{{ item.name }}</span>
            <span class="resource-rail__desc">{{ triggerLabel(item.trigger) }}</span>
          </span>
          <span class="resource-rail__tag" :class="item.enabled ? 'is-accent' : ''">
            {{ item.enabled ? $t('automations.enabled') : $t('automations.disabled') }}
          </span>
        </button>
      </nav>
    </template>

    <template #empty>
      <DqEmpty :description="$t('automations.emptySelection')">
        <p class="resource-workspace__hint">{{ $t('automations.emptySelectionHint') }}</p>
      </DqEmpty>
    </template>

    <template #header>
      <div class="resource-workspace__identity">
        <h1 class="resource-workspace__title">{{ headerTitle }}</h1>
        <div v-if="!isCreating && selected" class="resource-workspace__badges">
          <span class="resource-rail__tag" :class="selected.enabled ? 'is-accent' : ''">
            {{ selected.enabled ? $t('automations.enabled') : $t('automations.disabled') }}
          </span>
          <span class="resource-workspace__hint">{{ triggerLabel(selected.trigger) }}</span>
        </div>
      </div>
    </template>

    <template #body>
      <section class="resource-section">
        <div class="resource-form-grid resource-form-grid--2">
          <label class="resource-field">
            <span class="resource-field__label">{{ $t('common.name') }}</span>
            <DqInput v-model="form.name" :placeholder="$t('automations.dummyName')" />
          </label>
          <label class="resource-field">
            <span class="resource-field__label">{{ $t('automations.triggerType') }}</span>
            <DqSelect v-model="form.trigger" :placeholder="$t('automations.triggerType')">
              <DqOption v-for="opt in triggerOptions" :key="opt.value" :value="opt.value" :label="opt.label" />
            </DqSelect>
          </label>
        </div>

        <label class="resource-field resource-field--block">
          <span class="resource-field__label">{{ $t('common.description') }}</span>
          <DqInput v-model="form.description" type="textarea" :rows="3" :placeholder="$t('automations.descriptionPlaceholder')" />
        </label>

        <div v-if="form.trigger === 'schedule'" class="resource-form-grid resource-form-grid--2">
          <label class="resource-field">
            <span class="resource-field__label">{{ $t('automations.cronExpr') }}</span>
            <DqInput v-model="form.schedule" class="resource-input-mono" placeholder="0 9 * * *" />
            <span class="resource-field__hint">{{ $t('automations.cronHint') }}</span>
          </label>
        </div>

        <div v-if="form.trigger === 'event'" class="resource-form-grid resource-form-grid--2">
          <label class="resource-field">
            <span class="resource-field__label">{{ $t('automations.eventType') }}</span>
            <DqInput v-model="form.eventType" class="resource-input-mono" placeholder="alert.firing" />
          </label>
        </div>

        <div v-if="form.trigger === 'webhook'" class="resource-form-grid resource-form-grid--2">
          <label class="resource-field">
            <span class="resource-field__label">{{ $t('automations.webhookPath') }}</span>
            <DqInput v-model="form.webhookPath" class="resource-input-mono" placeholder="/webhooks/oncall" />
          </label>
        </div>
      </section>

      <section class="resource-section">
        <div class="resource-form-grid resource-form-grid--2">
          <label class="resource-field">
            <span class="resource-field__label">{{ $t('automations.execAgent') }}</span>
            <DqSelect v-model="form.agentId" :placeholder="$t('automations.selectAgent')" clearable>
              <DqOption v-for="agent in agents.items" :key="agent.id" :value="agent.id" :label="agent.name" />
            </DqSelect>
          </label>
        </div>

        <label class="resource-field resource-field--block">
          <span class="resource-field__label">{{ $t('automations.prompt_') }}</span>
          <DqInput v-model="form.prompt" type="textarea" :rows="8" :placeholder="$t('automations.promptPlaceholder')" />
        </label>
      </section>
    </template>

    <template #footer>
      <span class="resource-workspace__hint">{{ $t('common.saveShortcut') }}</span>
      <div class="resource-workspace__footer-actions">
        <DqButton v-if="isCreating" @click="isCreating = false; selectedId = null">{{ $t('common.cancel') }}</DqButton>
        <DqButton v-if="!isCreating" @click="runSelected">{{ $t('automations.runNow') }}</DqButton>
        <DqButton v-if="!isCreating" @click="toggleEnabled">
          {{ selected?.enabled ? $t('automations.disabled') : $t('automations.enabled') }}
        </DqButton>
        <DqButton v-if="!isCreating" @click="removeSelected">{{ $t('common.delete') }}</DqButton>
        <DqButton type="primary" :disabled="saving" @click="save">
          {{ isCreating ? $t('automations.createAutomation') : $t('common.save') }}
        </DqButton>
      </div>
    </template>
  </WorkspaceShell>
</template>

<style scoped>
.automations-filter {
  display: flex;
  gap: 4px;
  padding: 8px 10px 4px;
}

.automations-filter button {
  flex: 1;
  height: 28px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--dq-label-tertiary);
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  cursor: pointer;
}

.automations-filter button.is-active {
  background: color-mix(in srgb, var(--dq-accent) 12%, transparent);
  color: var(--dq-accent);
}
</style>
