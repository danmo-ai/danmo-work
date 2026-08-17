<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  payload: unknown
  decided: boolean
  deciding?: boolean
  showActions?: boolean
  anchorSeq?: number
}>()

const { t } = useI18n()

const emit = defineEmits<{
  decide: [payload: { decision: 'allow' | 'deny'; scope: 'once' | 'session' }]
}>()

function asRecord(v: unknown): Record<string, unknown> | null {
  if (v && typeof v === 'object' && !Array.isArray(v)) return v as Record<string, unknown>
  return null
}

function approvalReason(payload: unknown): string {
  return String(asRecord(payload)?.reason ?? '')
}

const reason = computed(() => approvalReason(props.payload))

const domain = computed(() => String(asRecord(props.payload)?.domain ?? ''))

const reasonLabel = computed(() => {
  switch (reason.value) {
    case 'network':
      return t('sessions.perm.needFullOutbound')
    case 'network_domain':
      return domain.value
        ? t('sessions.perm.allowDomain', { domain: domain.value })
        : t('sessions.perm.allowNewDomain')
    case 'dangerous_command':
      return t('sessions.perm.dangerousCommand')
    case 'unsandboxed':
      return t('sessions.perm.unsandboxed')
    default:
      return t('sessions.perm.needConfirm')
  }
})

const toolName = computed(() => {
  const p = asRecord(props.payload)
  return String(p?.tool ?? p?.name ?? t('sessions.perm.unknownTool'))
})

const description = computed(() => String(asRecord(props.payload)?.description ?? ''))

const allowsSession = computed(() => {
  const p = asRecord(props.payload)
  const opts = p?.scopeOptions
  if (Array.isArray(opts)) return opts.includes('session')
  return reason.value === 'network' || reason.value === 'network_domain'
})

const sessionButtonLabel = computed(() => {
  if (reason.value === 'network_domain') return t('sessions.perm.allowDomainSession')
  if (reason.value === 'network') return t('sessions.perm.allowFullOutboundSession')
  return t('sessions.perm.allowSession')
})

const pending = computed(() => Boolean(props.showActions) && !props.decided)
</script>

<template>
  <!-- Decided: quiet footnote — no tool-name wall -->
  <div
    v-if="decided && !pending"
    class="permission-ask permission-ask--settled"
    :data-event-anchor="anchorSeq"
    :title="`${reasonLabel}: ${toolName}`"
  >
    <span class="permission-ask__settled-dot" aria-hidden="true" />
    <span class="permission-ask__settled-text">{{ t('sessions.perm.settledWithReason', { reason: reasonLabel }) }}</span>
  </div>

  <div
    v-else
    class="permission-ask"
    :class="{ 'is-pending': pending }"
    :data-event-anchor="anchorSeq"
  >
    <div class="permission-ask__main">
      <svg class="permission-ask__icon" viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
        <path d="M7 11V7a5 5 0 0 1 10 0v4" />
      </svg>
      <div class="permission-ask__text">
        <span class="permission-ask__badge">{{ t('sessions.perm.needsAction') }}</span>
        <span>
          <strong>{{ reasonLabel }}</strong>：
          <strong>{{ toolName }}</strong>
          <template v-if="description"> — {{ description }}</template>
        </span>
      </div>
    </div>

    <div v-if="pending" class="permission-ask__actions">
      <DqButton type="primary" size="sm" :disabled="deciding" @click="emit('decide', { decision: 'allow', scope: 'once' })">
        {{ t('sessions.perm.allowOnce') }}
      </DqButton>
      <DqButton
        v-if="allowsSession"
        size="sm"
        :disabled="deciding"
        @click="emit('decide', { decision: 'allow', scope: 'session' })"
      >
        {{ sessionButtonLabel }}
      </DqButton>
      <DqButton size="sm" :disabled="deciding" @click="emit('decide', { decision: 'deny', scope: 'once' })">
        {{ t('sessions.perm.deny') }}
      </DqButton>
    </div>
    <p v-else-if="!decided" class="permission-ask__hint">{{ t('sessions.decideInComposer') }}</p>
  </div>
</template>

<style scoped>
.permission-ask {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 10px;
  border: 1px solid color-mix(in srgb, var(--dq-label-primary) 12%, transparent);
  background: color-mix(in srgb, var(--dq-label-primary) 3%, transparent);
  color: var(--dq-label-primary);
  font-size: var(--dq-font-size-body);
  scroll-margin-bottom: 96px;
}

.permission-ask.is-pending {
  border-color: color-mix(in srgb, var(--dq-warning, #d97706) 35%, transparent);
  background: color-mix(in srgb, var(--dq-warning, #d97706) 6%, transparent);
}

.permission-ask__main {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  flex: 1;
  min-width: 0;
}

.permission-ask__icon {
  flex-shrink: 0;
  margin-top: 2px;
  color: var(--dq-warning, #d97706);
  opacity: 0.85;
}

.permission-ask__text {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  min-width: 0;
  line-height: 1.45;
}

.permission-ask__badge {
  display: inline-flex;
  padding: 1px 7px;
  border-radius: 999px;
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  color: var(--dq-warning, #d97706);
  background: color-mix(in srgb, var(--dq-warning, #d97706) 14%, transparent);
}

.permission-ask__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-left: auto;
}

.permission-ask__hint {
  margin: 0;
  width: 100%;
  font-size: var(--dq-font-size-footnote);
  color: var(--dq-label-tertiary);
}

.permission-ask--settled {
  display: flex;
  align-items: center;
  gap: 7px;
  min-height: 22px;
  padding: 0;
  border: none;
  border-radius: 0;
  background: transparent;
  color: var(--dq-label-secondary);
  font-size: var(--dq-font-size-caption);
}

.permission-ask__settled-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  flex-shrink: 0;
  background: var(--dq-label-tertiary);
}

.permission-ask__settled-text {
  color: var(--dq-label-secondary);
}
</style>
