<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useLLMStore } from '@/stores/llm'
import { useProjectsStore } from '@/stores/projects'
import { useSessionsStore } from '@/stores/sessions'

const emit = defineEmits<{
  pickPrompt: [text: string]
}>()

const { t } = useI18n()
const router = useRouter()
const llm = useLLMStore()
const projects = useProjectsStore()
const sessions = useSessionsStore()

const hasModels = computed(() => llm.modelsLoaded && llm.models.length > 0)
const hasProjects = computed(() => projects.sortedProjects.length > 0)

const prompts = computed(() => [
  t('sessions.welcomePrompt1'),
  t('sessions.welcomePrompt2'),
  t('sessions.welcomePrompt3'),
])

function onPick(text: string) {
  emit('pickPrompt', text)
}

function goSettings() {
  router.push({ name: 'settings' })
}
</script>

<template>
  <div class="welcome-empty">
    <div class="welcome-empty__hero">
      <h2 class="welcome-empty__title">{{ t('sessions.welcomeTitle') }}</h2>
      <p class="welcome-empty__subtitle">{{ t('sessions.welcomeSubtitle') }}</p>
    </div>

    <div v-if="!hasModels" class="welcome-empty__alert">
      <p>{{ t('sessions.welcomeNeedModel') }}</p>
      <DqButton type="primary" size="sm" @click="goSettings">{{ t('sessions.welcomeConfigureModel') }}</DqButton>
    </div>

    <div v-else-if="!hasProjects" class="welcome-empty__alert">
      <p>{{ t('sessions.welcomeNeedProject') }}</p>
    </div>

    <div v-else class="welcome-empty__ready">
      <div class="welcome-empty__chips">
        <button
          v-for="(p, i) in prompts"
          :key="i"
          type="button"
          class="welcome-empty__chip"
          @click="onPick(p)"
        >
          {{ p }}
        </button>
      </div>
      <p class="welcome-empty__hint">
        {{ sessions.selectedProjectId
          ? t('sessions.welcomeHintReady')
          : t('sessions.welcomeHintPickProject') }}
      </p>
    </div>
  </div>
</template>

<style scoped>
.welcome-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 28px;
  min-height: 320px;
  padding: 48px 24px 140px;
  text-align: center;
}

.welcome-empty__hero {
  max-width: 480px;
}

.welcome-empty__title {
  margin: 0;
  font-size: var(--dq-welcome-title-size, var(--dq-font-size-hero));
  font-weight: 700;
  color: var(--dq-label-primary);
  letter-spacing: -0.03em;
  line-height: var(--dq-line-height-tight);
}

.welcome-empty__subtitle {
  margin: 12px 0 0;
  font-size: var(--dq-font-size-secondary);
  color: var(--dq-label-secondary);
  line-height: 1.55;
}

/* Neutral elevated setup card — orange washes read muddy on dark glass */
.welcome-empty__alert {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  max-width: 380px;
  padding: 20px 24px;
  border-radius: 14px;
  background: var(--dq-glass-grouped-bg, var(--dq-bg-elevated));
  border: 1px solid var(--dq-glass-border, var(--dq-border));
  -webkit-backdrop-filter: var(--dq-glass-blur-light);
  backdrop-filter: var(--dq-glass-blur-light);
  box-shadow: var(--dq-shadow-md);
  color: var(--dq-label-secondary);
  font-size: var(--dq-font-size-body);
  line-height: 1.5;
}

.welcome-empty__alert p {
  margin: 0;
}

.welcome-empty__ready {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
  max-width: 560px;
  width: 100%;
}

.welcome-empty__chips {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 10px;
}

.welcome-empty__chip {
  max-width: 300px;
  padding: 10px 14px;
  border-radius: 12px;
  border: 1px solid var(--dq-welcome-chip-border);
  background: var(--dq-welcome-chip-bg);
  color: var(--dq-label-primary);
  font-size: var(--dq-font-size-footnote);
  line-height: 1.45;
  text-align: left;
  cursor: pointer;
  transition: border-color 0.15s ease, background 0.15s ease, transform 0.12s ease, box-shadow 0.15s ease;
}

.welcome-empty__chip:hover {
  border-color: color-mix(in srgb, var(--dq-accent) 40%, transparent);
  background: var(--dq-welcome-chip-hover);
  transform: translateY(-1px);
  box-shadow: var(--dq-shadow-sm);
}

.welcome-empty__chip:active {
  transform: scale(0.98);
}

.welcome-empty__hint {
  margin: 0;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-tertiary);
}
</style>
