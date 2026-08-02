<script setup lang="ts">
defineProps<{
  html: string
}>()
</script>

<template>
  <div class="agent-msg dq-prose dq-msg-enter" v-html="html" />
</template>

<style scoped>
.agent-msg {
  padding: 4px 0 6px;
  color: var(--dq-msg-agent-fg, var(--dq-label-primary));
  font-size: var(--dq-font-size-body);
  line-height: var(--dq-line-height-prose, 1.5);
  font-weight: 400;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

/* v-html children must inherit — do not let tertiary/muted bleed in */
.agent-msg :deep(p),
.agent-msg :deep(li),
.agent-msg :deep(td),
.agent-msg :deep(th),
.agent-msg :deep(blockquote) {
  color: inherit;
  font-size: inherit;
  font-weight: inherit;
}

.agent-msg :deep(strong) {
  font-weight: 650;
  color: inherit;
}

.agent-msg :deep(a) {
  color: var(--dq-accent);
}

.agent-msg :deep(> :first-child) {
  margin-top: 0;
}

.agent-msg :deep(> :last-child) {
  margin-bottom: 0;
}
</style>
