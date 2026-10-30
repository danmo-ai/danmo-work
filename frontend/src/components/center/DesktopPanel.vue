<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { fetchJSON } from '@/api/client'
import type { EnvironmentStatus } from '@/types/mission'

const props = defineProps<{
  projectId: string | null
  active?: boolean
}>()

const { t } = useI18n()
const status = ref<EnvironmentStatus | null>(null)
const errorText = ref('')
const loading = ref(false)
let timer: ReturnType<typeof setInterval> | null = null

const novncURL = computed(() => {
  const u = status.value?.novncUrl?.trim()
  if (!u) return ''
  // autofit + view-only keeps the agent in control unless the user wants to intervene.
  const join = u.includes('?') ? '&' : '?'
  return `${u}${join}autoconnect=1&resize=scale&view_only=0`
})

const isDesktopProfile = computed(() => status.value?.profile === 'desktop')
const geometry = computed(() => {
  const w = status.value?.desktopWidth
  const h = status.value?.desktopHeight
  if (w && h) return `${w}×${h}`
  return ''
})

async function refresh() {
  loading.value = true
  errorText.value = ''
  try {
    status.value = await fetchJSON<EnvironmentStatus>('/environment/status')
  } catch (e) {
    errorText.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

function startPoll() {
  stopPoll()
  void refresh()
  timer = setInterval(() => {
    if (props.active !== false) void refresh()
  }, 4000)
}

function stopPoll() {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

onMounted(startPoll)
onBeforeUnmount(stopPoll)

watch(
  () => props.active,
  (active) => {
    if (active === false) stopPoll()
    else startPoll()
  },
)

defineExpose({ refresh })
</script>

<template>
  <div class="desktop-panel">
    <div class="desktop-panel__toolbar">
      <span class="desktop-panel__hint">
        <template v-if="geometry">{{ t('sessions.desktop.geometry', { size: geometry }) }}</template>
        <template v-else>{{ t('sessions.desktop.title') }}</template>
      </span>
      <button type="button" class="desktop-panel__btn" :disabled="loading" @click="refresh">
        {{ t('sessions.desktop.refresh') }}
      </button>
      <a
        v-if="novncURL"
        class="desktop-panel__btn desktop-panel__btn--link"
        :href="novncURL"
        target="_blank"
        rel="noopener noreferrer"
      >
        {{ t('sessions.desktop.openExternal') }}
      </a>
    </div>

    <div v-if="errorText" class="desktop-panel__empty">{{ errorText }}</div>
    <div v-else-if="!isDesktopProfile" class="desktop-panel__empty">
      {{ t('sessions.desktop.needProfile') }}
    </div>
    <div v-else-if="!novncURL" class="desktop-panel__empty">
      {{ t('sessions.desktop.waiting') }}
    </div>
    <iframe
      v-else
      class="desktop-panel__frame"
      :src="novncURL"
      :title="t('sessions.desktop.title')"
      allow="clipboard-read; clipboard-write"
    />
  </div>
</template>

<style scoped>
.desktop-panel {
  display: flex;
  flex-direction: column;
  min-height: 0;
  flex: 1;
  height: 100%;
}

.desktop-panel__toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--dq-separator, rgba(127, 127, 127, 0.25));
  flex-shrink: 0;
}

.desktop-panel__hint {
  flex: 1;
  min-width: 0;
  font-size: var(--dq-font-size-caption, 12px);
  color: var(--dq-label-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.desktop-panel__btn {
  border: 1px solid var(--dq-separator, rgba(127, 127, 127, 0.35));
  background: transparent;
  color: var(--dq-label-primary);
  border-radius: 6px;
  padding: 2px 8px;
  font-size: var(--dq-font-size-caption, 12px);
  cursor: pointer;
}

.desktop-panel__btn:disabled {
  opacity: 0.5;
  cursor: default;
}

.desktop-panel__btn--link {
  text-decoration: none;
  display: inline-flex;
  align-items: center;
}

.desktop-panel__empty {
  padding: 24px 16px;
  font-size: var(--dq-font-size-body);
  color: var(--dq-label-tertiary);
  text-align: center;
}

.desktop-panel__frame {
  flex: 1;
  min-height: 0;
  width: 100%;
  border: 0;
  background: #111;
}
</style>
