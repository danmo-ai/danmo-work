<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { NovelCastCard, NovelCastRole } from '@/types/novel-workbench'

const props = defineProps<{
  cards: NovelCastCard[]
}>()

const emit = defineEmits<{
  open: [stem: string]
}>()

const { t } = useI18n()

const ROLE_ORDER: NovelCastRole[] = ['protagonist', 'volume_antagonist', 'recurring', '']

function roleLabel(role: NovelCastRole): string {
  switch (role) {
    case 'protagonist':
      return t('novelWorkbench.roleProtagonist')
    case 'volume_antagonist':
      return t('novelWorkbench.roleAntagonist')
    case 'recurring':
      return t('novelWorkbench.roleRecurring')
    default:
      return t('novelWorkbench.roleUnset')
  }
}

function statusLabel(card: NovelCastCard): string {
  if (card.status === 'canon') return t('novelWorkbench.castStatusCanon')
  if (card.status === 'candidate') return t('novelWorkbench.castStatusCandidate')
  return t('novelWorkbench.castStatusMissing')
}

const groups = computed(() =>
  ROLE_ORDER.map((role) => ({
    role,
    label: roleLabel(role),
    cards: props.cards.filter((c) => c.role === role),
  })).filter((g) => g.cards.length),
)
</script>

<template>
  <div class="novel-cast">
    <p v-if="!cards.length" class="novel-cast__empty">{{ t('novelWorkbench.noCastYet') }}</p>
    <div v-for="g in groups" :key="g.role || 'unset'" class="novel-cast__group">
      <div class="novel-cast__role">{{ g.label }}</div>
      <div class="novel-cast__grid">
        <button
          v-for="card in g.cards"
          :key="card.stem"
          type="button"
          class="novel-cast__card"
          @click="emit('open', card.stem)"
        >
          <div class="novel-cast__top">
            <span class="novel-cast__name">{{ card.name || card.stem }}</span>
            <span class="novel-cast__status" :class="'novel-cast__status--' + (card.status || 'missing')">
              {{ statusLabel(card) }}
            </span>
          </div>
          <div class="novel-cast__stem">{{ card.stem }}</div>
          <p v-if="card.visualAnchor" class="novel-cast__line">{{ card.visualAnchor }}</p>
          <p v-if="card.catchphrase" class="novel-cast__line novel-cast__line--quote">「{{ card.catchphrase }}」</p>
          <p v-if="card.missing.length" class="novel-cast__missing">
            {{ t('novelWorkbench.castMissing', { n: card.missing.length }) }}
            <span>{{ card.missing.join(' · ') }}</span>
          </p>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.novel-cast {
  padding: 4px 0 24px;
}

.novel-cast__empty {
  margin: 0;
  opacity: 0.65;
}

.novel-cast__group + .novel-cast__group {
  margin-top: 16px;
}

.novel-cast__role {
  margin-bottom: 8px;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.04em;
  opacity: 0.6;
}

.novel-cast__grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 8px;
}

.novel-cast__card {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 4px;
  margin: 0;
  padding: 10px 12px;
  border: 1px solid color-mix(in srgb, var(--dq-border-subtle, #000) 45%, transparent);
  border-radius: 8px;
  background: color-mix(in srgb, var(--dq-glass-popover-bg, #fff) 70%, transparent);
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.novel-cast__card:hover {
  border-color: color-mix(in srgb, var(--dq-accent) 45%, transparent);
}

.novel-cast__top {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
}

.novel-cast__name {
  font-weight: 650;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.novel-cast__stem {
  font-size: 11px;
  opacity: 0.5;
}

.novel-cast__status {
  flex-shrink: 0;
  padding: 1px 6px;
  border-radius: 999px;
  font-size: 10px;
  font-weight: 650;
  background: color-mix(in srgb, var(--dq-border-subtle, #000) 25%, transparent);
}

.novel-cast__status--canon {
  background: color-mix(in srgb, var(--dq-success, #16a34a) 16%, transparent);
  color: var(--dq-success, #16a34a);
}

.novel-cast__status--candidate {
  background: color-mix(in srgb, #ca8a04 16%, transparent);
  color: #a16207;
}

.novel-cast__line {
  margin: 0;
  font-size: var(--dq-font-size-caption);
  line-height: 1.4;
  opacity: 0.85;
}

.novel-cast__line--quote {
  opacity: 0.7;
}

.novel-cast__missing {
  margin: 2px 0 0;
  font-size: 11px;
  color: var(--dq-danger, #dc2626);
  line-height: 1.35;
}

.novel-cast__missing span {
  display: block;
  font-weight: 450;
  opacity: 0.85;
}
</style>
