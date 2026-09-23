<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { buildRelationEdges, type NovelCastCard, type NovelCastRole } from '@/types/novel-workbench'

const props = defineProps<{
  cards: NovelCastCard[]
}>()

const emit = defineEmits<{
  open: [stem: string]
}>()

const { t } = useI18n()

const W = 640
const H = 420
const MAX_NODES = 30

type Point = { x: number; y: number; role: NovelCastRole | 'missing'; label: string }

const layout = computed(() => {
  const known = new Map(props.cards.map((c) => [c.stem, c]))
  const edges = buildRelationEdges(props.cards)
  const columns: { key: string; stems: string[] }[] = [
    { key: 'protagonist', stems: [] },
    { key: 'recurring', stems: [] },
    { key: 'volume_antagonist', stems: [] },
    { key: '', stems: [] },
  ]
  const placed = new Set<string>()
  const take = (stem: string, key: string) => {
    if (placed.size >= MAX_NODES || placed.has(stem)) return
    placed.add(stem)
    ;(columns.find((c) => c.key === key) ?? columns[3]).stems.push(stem)
  }
  for (const card of props.cards) take(card.stem, card.role)
  for (const edge of edges) {
    if (!known.has(edge.to)) take(edge.to, '')
  }
  const active = columns.filter((c) => c.stems.length)
  const positions = new Map<string, Point>()
  active.forEach((col, i) => {
    const x = active.length === 1 ? W / 2 : 88 + (i * (W - 176)) / Math.max(1, active.length - 1)
    col.stems.forEach((stem, j) => {
      const y = col.stems.length === 1 ? H / 2 : 48 + (j * (H - 96)) / Math.max(1, col.stems.length - 1)
      const card = known.get(stem)
      positions.set(stem, {
        x,
        y,
        role: card ? card.role : 'missing',
        label: card?.name || stem,
      })
    })
  })
  const lines = edges
    .map((edge, index) => {
      const a = positions.get(edge.from)
      const b = positions.get(edge.to)
      if (!a || !b) return null
      const dx = b.x - a.x
      const dy = b.y - a.y
      const len = Math.hypot(dx, dy) || 1
      const k = 5
      const ox = (-dy / len) * k
      const oy = (dx / len) * k
      return {
        ...edge,
        index,
        x1: a.x + ox,
        y1: a.y + oy,
        x2: b.x + ox,
        y2: b.y + oy,
      }
    })
    .filter((line): line is NonNullable<typeof line> => Boolean(line))
  return {
    positions: [...positions.entries()].map(([stem, point]) => ({ stem, ...point })),
    lines,
    truncated: props.cards.length > MAX_NODES,
  }
})

function nodeClass(role: NovelCastRole | 'missing'): string {
  if (role === 'missing') return 'novel-rel__node--missing'
  if (role === 'protagonist') return 'novel-rel__node--protagonist'
  if (role === 'volume_antagonist') return 'novel-rel__node--antagonist'
  return 'novel-rel__node--recurring'
}

function openStem(stem: string) {
  if (props.cards.some((c) => c.stem === stem)) emit('open', stem)
}
</script>

<template>
  <div class="novel-rel">
    <p v-if="!cards.length" class="novel-rel__empty">{{ t('novelWorkbench.noCastYet') }}</p>
    <template v-else>
      <p v-if="layout.truncated" class="novel-rel__note">{{ t('novelWorkbench.relationTruncated', { n: MAX_NODES }) }}</p>
      <svg class="novel-rel__svg" :viewBox="`0 0 ${W} ${H}`" role="img" :aria-label="t('novelWorkbench.castGraph')">
        <line
          v-for="line in layout.lines"
          :key="line.index"
          :x1="line.x1"
          :y1="line.y1"
          :x2="line.x2"
          :y2="line.y2"
          class="novel-rel__edge"
          :class="{
            'novel-rel__edge--gap': line.missingBackEdge,
            'novel-rel__edge--unknown': line.unknownTarget,
          }"
        >
          <title>{{ line.from }} → {{ line.to }}{{ line.current ? ' · ' + line.current : '' }}</title>
        </line>
        <g
          v-for="node in layout.positions"
          :key="node.stem"
          class="novel-rel__node"
          :class="nodeClass(node.role)"
          @click="openStem(node.stem)"
        >
          <circle :cx="node.x" :cy="node.y" r="7" />
          <text :x="node.x" :y="node.y + 20" text-anchor="middle">{{ node.label }}</text>
        </g>
      </svg>
      <p class="novel-rel__legend">
        <span class="novel-rel__swatch novel-rel__swatch--solid" /> {{ t('novelWorkbench.relationOk') }}
        <span class="novel-rel__swatch novel-rel__swatch--gap" /> {{ t('novelWorkbench.relationMissing') }}
      </p>
      <p v-if="!layout.lines.length" class="novel-rel__empty">{{ t('novelWorkbench.noRelations') }}</p>
    </template>
  </div>
</template>

<style scoped>
.novel-rel__empty,
.novel-rel__note,
.novel-rel__legend {
  margin: 0 0 8px;
  font-size: var(--dq-font-size-caption);
  opacity: 0.7;
}

.novel-rel__svg {
  width: 100%;
  height: auto;
  max-height: 440px;
}

.novel-rel__edge {
  stroke: color-mix(in srgb, var(--dq-accent) 70%, currentColor);
  stroke-width: 1.4;
}

.novel-rel__edge--gap {
  stroke-dasharray: 4 3;
  stroke: color-mix(in srgb, var(--dq-danger, #dc2626) 75%, currentColor);
}

.novel-rel__edge--unknown {
  stroke-dasharray: 2 3;
  stroke: color-mix(in srgb, var(--dq-danger, #dc2626) 90%, currentColor);
}

.novel-rel__node {
  cursor: pointer;
}

.novel-rel__node circle {
  fill: color-mix(in srgb, var(--dq-accent) 80%, #888);
}

.novel-rel__node--antagonist circle {
  fill: color-mix(in srgb, var(--dq-danger, #dc2626) 75%, #888);
}

.novel-rel__node--recurring circle {
  fill: color-mix(in srgb, currentColor 55%, transparent);
}

.novel-rel__node--missing circle {
  fill: transparent;
  stroke: currentColor;
  stroke-dasharray: 2 2;
}

.novel-rel__node text {
  fill: currentColor;
  font-size: 11px;
}

.novel-rel__legend {
  display: flex;
  align-items: center;
  gap: 6px;
}

.novel-rel__swatch {
  display: inline-block;
  width: 18px;
  height: 0;
  margin-left: 8px;
  border-top: 2px solid color-mix(in srgb, var(--dq-accent) 70%, currentColor);
}

.novel-rel__swatch--gap {
  border-top-style: dashed;
  border-top-color: var(--dq-danger, #dc2626);
}
</style>
