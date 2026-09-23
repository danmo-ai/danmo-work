<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { type NovelCastCard, type NovelCastRole } from '@/types/novel-workbench'

const props = defineProps<{
  cards: NovelCastCard[]
}>()

const emit = defineEmits<{
  open: [stem: string]
}>()

const { t } = useI18n()

const MAX_NODES = 30
const MAX_PER_RING = 11
const MIN_ARC = 86
const RING_GAP = 108
const LABEL_PAD = 56
const MIN_CANVAS = 560
const DRAG_PX = 3

type Point = {
  x: number
  y: number
  role: NovelCastRole | 'missing'
  label: string
  title: string
  tx: number
  ty: number
  anchor: 'start' | 'middle' | 'end'
}
type DrawEdge = {
  key: string
  from: string
  to: string
  fromLabel: string
  toLabel: string
  current: string
  missingBackEdge: boolean
  unknownTarget: boolean
}
type View = { x: number; y: number; w: number; h: number }
type Gesture =
  | { kind: 'node'; pointerId: number; stem: string; sx: number; sy: number; ox: number; oy: number; moved: boolean }
  | { kind: 'pan'; pointerId: number; cx: number; cy: number; vx: number; vy: number }

const svgRef = ref<SVGSVGElement | null>(null)
const view = ref<View>({ x: 0, y: 0, w: MIN_CANVAS, h: MIN_CANVAS })
const offsets = ref<Record<string, { x: number; y: number }>>({})
const gesture = ref<Gesture | null>(null)

function normName(value: string): string {
  return value.replace(/\s+/g, '').trim()
}

function shortLabel(value: string): string {
  const chars = [...value.trim()]
  if (chars.length <= 8) return chars.join('')
  return chars.slice(0, 7).join('') + '…'
}

function circMean(angles: number[]): number {
  if (!angles.length) return -Math.PI / 2
  let x = 0
  let y = 0
  for (const angle of angles) {
    x += Math.cos(angle)
    y += Math.sin(angle)
  }
  if (x === 0 && y === 0) return angles[0] ?? -Math.PI / 2
  return Math.atan2(y, x)
}

function layoutRadial(
  ids: string[],
  adj: Map<string, string[]>,
  hub?: string,
): { at: Map<string, { x: number; y: number }>; radius: number } {
  const at = new Map<string, { x: number; y: number }>()
  if (!ids.length) return { at, radius: 0 }
  if (ids.length === 1) {
    at.set(ids[0], { x: 0, y: 0 })
    return { at, radius: 0 }
  }
  const idSet = new Set(ids)
  const start = hub && idSet.has(hub)
    ? hub
    : [...ids].sort((a, b) => (adj.get(b)?.length ?? 0) - (adj.get(a)?.length ?? 0) || a.localeCompare(b, 'zh'))[0]
  const layers: string[][] = [[start]]
  const seen = new Set([start])
  let frontier = [start]
  while (frontier.length) {
    const next: string[] = []
    for (const id of frontier) {
      const neigh = [...(adj.get(id) ?? [])].filter((n) => idSet.has(n) && !seen.has(n))
      neigh.sort((a, b) => a.localeCompare(b, 'zh'))
      for (const n of neigh) {
        if (seen.has(n)) continue
        seen.add(n)
        next.push(n)
      }
    }
    if (next.length) layers.push(next)
    frontier = next
  }
  const rest = ids.filter((id) => !seen.has(id)).sort((a, b) => a.localeCompare(b, 'zh'))
  if (rest.length) layers.push(rest)

  const packed: string[][] = []
  if (layers[0]) packed.push(layers[0])
  for (let i = 1; i < layers.length; i++) {
    const layer = layers[i]
    const chunks = Math.max(1, Math.ceil(layer.length / MAX_PER_RING))
    const size = Math.ceil(layer.length / chunks)
    for (let k = 0; k < layer.length; k += size) packed.push(layer.slice(k, k + size))
  }
  layers.splice(0, layers.length, ...packed)
  while (
    layers.length > 2
    && layers[layers.length - 1].length <= 2
    && layers[layers.length - 2].length + layers[layers.length - 1].length <= MAX_PER_RING
  ) {
    const extra = layers.pop()!
    layers[layers.length - 1].push(...extra)
  }

  const angle = new Map<string, number>()
  const spread = (layer: string[], mean: number) => {
    if (layer.length === 1) {
      angle.set(layer[0], mean)
      return
    }
    const startAng = mean - Math.PI + Math.PI / layer.length
    layer.forEach((id, i) => angle.set(id, startAng + (i * 2 * Math.PI) / layer.length))
  }
  for (const layer of layers) spread(layer, -Math.PI / 2)
  for (let sweep = 0; sweep < 6; sweep++) {
    for (let li = 1; li < layers.length; li++) {
      const scored = layers[li].map((id) => {
        const neigh = (adj.get(id) ?? []).filter((n) => angle.has(n))
        const ang = neigh.length ? circMean(neigh.map((n) => angle.get(n)!)) : (angle.get(id) ?? 0)
        return { id, ang }
      })
      const mean = circMean(scored.map((s) => s.ang))
      scored.sort((a, b) => {
        const da = Math.atan2(Math.sin(a.ang - mean), Math.cos(a.ang - mean))
        const db = Math.atan2(Math.sin(b.ang - mean), Math.cos(b.ang - mean))
        return da - db || a.id.localeCompare(b.id, 'zh')
      })
      layers[li] = scored.map((s) => s.id)
      spread(layers[li], circMean(scored.map((s) => s.ang)))
    }
  }

  const radius = [0]
  for (let i = 1; i < layers.length; i++) {
    const need = (layers[i].length * MIN_ARC) / (2 * Math.PI)
    radius[i] = Math.max(radius[i - 1] + RING_GAP, need)
  }
  layers.forEach((layer, li) => {
    for (const id of layer) {
      const a = angle.get(id) ?? 0
      const r = radius[li] ?? 0
      at.set(id, { x: Math.cos(a) * r, y: Math.sin(a) * r })
    }
  })
  return { at, radius: radius[radius.length - 1] ?? 0 }
}

const base = computed(() => {
  const byStem = new Map(props.cards.map((c) => [c.stem, c]))
  const nameHits = new Map<string, string[]>()
  for (const card of props.cards) {
    const name = normName(card.name)
    if (!name) continue
    const list = nameHits.get(name) ?? []
    list.push(card.stem)
    nameHits.set(name, list)
  }
  const byName = new Map<string, string>()
  for (const [name, stems] of nameHits) {
    if (stems.length === 1) byName.set(name, stems[0])
  }
  const resolve = (target: string) => {
    if (byStem.has(target)) return target
    const exact = byName.get(normName(target))
    if (exact) return exact
    const name = normName(target)
    if (name.length < 2) return target
    const hits = props.cards.filter((card) => {
      const cardName = normName(card.name)
      if (!cardName.startsWith(name) || cardName.length === name.length) return false
      return '·（('.includes(cardName[name.length] ?? '')
    })
    return hits.length === 1 ? hits[0].stem : target
  }

  const ids: string[] = []
  const idSet = new Set<string>()
  const take = (id: string) => {
    if (!id || idSet.has(id) || idSet.size >= MAX_NODES) return
    idSet.add(id)
    ids.push(id)
  }
  for (const card of props.cards) take(card.stem)
  for (const card of props.cards) {
    for (const rel of card.relations) take(resolve(rel.target))
  }

  const linksTo = (card: NovelCastCard, other: NovelCastCard) => card.relations.some((rel) => {
    const id = resolve(rel.target)
    return id === other.stem
  })
  const labelOf = (id: string) => byStem.get(id)?.name || id
  const edges: DrawEdge[] = []
  const seenEdge = new Map<string, DrawEdge>()
  for (const card of props.cards) {
    if (!idSet.has(card.stem)) continue
    for (const rel of card.relations) {
      const to = resolve(rel.target)
      if (!to || to === card.stem || !idSet.has(to)) continue
      const key = [card.stem, to].sort((a, b) => a.localeCompare(b)).join('\0')
      const ghost = !byStem.has(to)
      const other = byStem.get(to)
      const back = Boolean(other && linksTo(other, card))
      const existing = seenEdge.get(key)
      if (!existing) {
        const edge: DrawEdge = {
          key,
          from: card.stem,
          to,
          fromLabel: labelOf(card.stem),
          toLabel: labelOf(to),
          current: rel.current,
          unknownTarget: ghost,
          missingBackEdge: !ghost && !back,
        }
        seenEdge.set(key, edge)
        edges.push(edge)
      } else if (back || existing.from === to) {
        existing.missingBackEdge = false
        if (!existing.current && rel.current) existing.current = rel.current
      }
    }
  }

  const adj = new Map<string, string[]>()
  for (const id of ids) adj.set(id, [])
  for (const edge of edges) {
    adj.get(edge.from)?.push(edge.to)
    adj.get(edge.to)?.push(edge.from)
  }

  const hubCard = props.cards.find((c) => c.role === 'protagonist' && idSet.has(c.stem))
    ?? [...byStem.values()]
      .filter((c) => idSet.has(c.stem))
      .sort((a, b) => (adj.get(b.stem)?.length ?? 0) - (adj.get(a.stem)?.length ?? 0) || a.stem.localeCompare(b.stem))[0]
  const hub = hubCard?.stem
  const seen = new Set<string>()
  const components: string[][] = []
  const starts = hub ? [hub, ...ids.filter((id) => id !== hub)] : ids
  for (const start of starts) {
    if (seen.has(start)) continue
    const comp: string[] = []
    const queue = [start]
    seen.add(start)
    while (queue.length) {
      const id = queue.shift()!
      comp.push(id)
      for (const n of adj.get(id) ?? []) {
        if (seen.has(n)) continue
        seen.add(n)
        queue.push(n)
      }
    }
    components.push(comp)
  }
  components.sort((a, b) => b.length - a.length || a[0].localeCompare(b[0], 'zh'))
  if (hub) {
    const hi = components.findIndex((comp) => comp.includes(hub))
    if (hi > 0) {
      const [main] = components.splice(hi, 1)
      components.unshift(main)
    }
  }

  const mainIds = [...(components[0] ?? [])]
  const satellites: string[][] = []
  for (const comp of components.slice(1)) {
    if (comp.length === 1) mainIds.push(comp[0])
    else satellites.push(comp)
  }
  const laidMain = layoutRadial(mainIds, adj, mainIds.includes(hub ?? '') ? hub : undefined)
  const laidSats = satellites.map((comp) => layoutRadial(comp, adj, comp.includes(hub ?? '') ? hub : undefined))
  const local = new Map<string, { x: number; y: number }>()
  let outer = laidMain.radius
  for (const [id, p] of laidMain.at) local.set(id, { ...p })
  if (laidSats.length) {
    const satR = laidSats.map((s) => s.radius)
    const need = (laidSats.length * MIN_ARC) / (2 * Math.PI)
    const orbit = Math.max(laidMain.radius + Math.max(...satR, 28) + RING_GAP, need)
    laidSats.forEach((sat, i) => {
      const a = -Math.PI / 2 + (i * 2 * Math.PI) / laidSats.length
      const cx = Math.cos(a) * orbit
      const cy = Math.sin(a) * orbit
      for (const [id, p] of sat.at) local.set(id, { x: p.x + cx, y: p.y + cy })
      outer = Math.max(outer, orbit + sat.radius)
    })
  }
  const size = Math.max(MIN_CANVAS, (outer + LABEL_PAD) * 2)
  const positions = new Map<string, Point>()
  for (const id of ids) {
    const p = local.get(id) ?? { x: 0, y: 0 }
    const x = p.x + size / 2
    const y = p.y + size / 2
    const card = byStem.get(id)
    const title = labelOf(id)
    const vx = x - size / 2
    const vy = y - size / 2
    const len = Math.hypot(vx, vy)
    let tx = x
    let ty = y + 18
    let anchor: Point['anchor'] = 'middle'
    if (len >= 8) {
      const ux = vx / len
      const uy = vy / len
      tx = x + ux * 16
      ty = y + uy * 16 + 4
      anchor = ux > 0.35 ? 'start' : ux < -0.35 ? 'end' : 'middle'
    }
    positions.set(id, {
      x,
      y,
      role: card ? card.role : 'missing',
      label: shortLabel(title),
      title,
      tx,
      ty,
      anchor,
    })
  }
  return {
    positions,
    edges,
    truncated: props.cards.length > MAX_NODES || props.cards.some((c) => c.relations.some((r) => !idSet.has(resolve(r.target)) && resolve(r.target) !== c.stem)),
    width: size,
    height: size,
  }
})

watch(
  () => [...base.value.positions.keys()].join('\0'),
  () => {
    const keep = new Set(base.value.positions.keys())
    const next: Record<string, { x: number; y: number }> = {}
    for (const [stem, point] of Object.entries(offsets.value)) {
      if (keep.has(stem)) next[stem] = point
    }
    offsets.value = next
  },
)

function pointOf(stem: string): Point | undefined {
  const origin = base.value.positions.get(stem)
  if (!origin) return undefined
  const moved = offsets.value[stem]
  if (!moved) return origin
  return {
    ...origin,
    x: moved.x,
    y: moved.y,
    tx: origin.tx + (moved.x - origin.x),
    ty: origin.ty + (moved.y - origin.y),
  }
}

function trimEnds(a: Point, b: Point, pad: number): { x: number; y: number }[] {
  const dx = b.x - a.x
  const dy = b.y - a.y
  const len = Math.hypot(dx, dy) || 1
  const ux = dx / len
  const uy = dy / len
  const cut = Math.min(pad, len / 3)
  return [
    { x: a.x + ux * cut, y: a.y + uy * cut },
    { x: b.x - ux * cut, y: b.y - uy * cut },
  ]
}

function orient(ax: number, ay: number, bx: number, by: number, cx: number, cy: number): number {
  return (bx - ax) * (cy - ay) - (by - ay) * (cx - ax)
}

function properCross(
  a: { x: number; y: number },
  b: { x: number; y: number },
  c: { x: number; y: number },
  d: { x: number; y: number },
): boolean {
  const o1 = orient(a.x, a.y, b.x, b.y, c.x, c.y)
  const o2 = orient(a.x, a.y, b.x, b.y, d.x, d.y)
  const o3 = orient(c.x, c.y, d.x, d.y, a.x, a.y)
  const o4 = orient(c.x, c.y, d.x, d.y, b.x, b.y)
  return o1 * o2 < 0 && o3 * o4 < 0
}

function curvePath(a: Point, b: Point, bend: number): string {
  const [p, q] = trimEnds(a, b, 11)
  if (Math.abs(bend) < 1) return `M ${p.x} ${p.y} L ${q.x} ${q.y}`
  const mx = (p.x + q.x) / 2
  const my = (p.y + q.y) / 2
  const dx = q.x - p.x
  const dy = q.y - p.y
  const len = Math.hypot(dx, dy) || 1
  return `M ${p.x} ${p.y} Q ${mx + (-dy / len) * bend} ${my + (dx / len) * bend} ${q.x} ${q.y}`
}

const scene = computed(() => {
  const positions = [...base.value.positions.keys()]
    .map((stem) => {
      const point = pointOf(stem)
      return point ? { stem, ...point } : null
    })
    .filter((node): node is { stem: string } & Point => Boolean(node))
  const byStem = new Map(positions.map((node) => [node.stem, node]))
  const raw = base.value.edges
    .map((edge) => {
      const a = byStem.get(edge.from)
      const b = byStem.get(edge.to)
      if (!a || !b) return null
      const [p, q] = trimEnds(a, b, 14)
      return { edge, a, b, p, q }
    })
    .filter((line): line is NonNullable<typeof line> => Boolean(line))
  const bend = new Map<string, number>()
  for (let i = 0; i < raw.length; i++) {
    for (let j = i + 1; j < raw.length; j++) {
      const left = raw[i]
      const right = raw[j]
      if (!properCross(left.p, left.q, right.p, right.q)) continue
      const li = Math.hypot(left.q.x - left.p.x, left.q.y - left.p.y)
      const lj = Math.hypot(right.q.x - right.p.x, right.q.y - right.p.y)
      const pick = li >= lj ? left : right
      const sign = (bend.get(pick.edge.key) ?? ((i + j) % 2 === 0 ? 34 : -34)) > 0 ? 1 : -1
      bend.set(pick.edge.key, sign * Math.min(64, Math.abs(bend.get(pick.edge.key) ?? 28) + 16))
    }
  }
  const lines = raw.map((line) => ({
    ...line.edge,
    d: curvePath(line.a, line.b, bend.get(line.edge.key) ?? 0),
  }))
  return { positions, lines, truncated: base.value.truncated }
})

const viewBox = computed(() => `${view.value.x} ${view.value.y} ${view.value.w} ${view.value.h}`)

function nodeClass(role: NovelCastRole | 'missing'): string {
  if (role === 'missing') return 'novel-rel__node--missing'
  if (role === 'protagonist') return 'novel-rel__node--protagonist'
  if (role === 'volume_antagonist') return 'novel-rel__node--antagonist'
  return 'novel-rel__node--recurring'
}

function openStem(stem: string) {
  if (props.cards.some((c) => c.stem === stem)) emit('open', stem)
}

function clientToSvg(e: { clientX: number; clientY: number }): { x: number; y: number } | null {
  const svg = svgRef.value
  const ctm = svg?.getScreenCTM()
  if (!svg || !ctm) return null
  const pt = svg.createSVGPoint()
  pt.x = e.clientX
  pt.y = e.clientY
  const p = pt.matrixTransform(ctm.inverse())
  return { x: p.x, y: p.y }
}

function zoomAt(svgX: number, svgY: number, factor: number) {
  const v = view.value
  const width = base.value.width
  const nextW = Math.min(width * 4, Math.max(width / 4, v.w * factor))
  const nextH = nextW * (base.value.height / width)
  const kx = (svgX - v.x) / v.w
  const ky = (svgY - v.y) / v.h
  view.value = {
    x: svgX - kx * nextW,
    y: svgY - ky * nextH,
    w: nextW,
    h: nextH,
  }
}

function zoomCenter(factor: number) {
  const v = view.value
  zoomAt(v.x + v.w / 2, v.y + v.h / 2, factor)
}

function resetView() {
  view.value = { x: 0, y: 0, w: base.value.width, h: base.value.height }
  offsets.value = {}
  gesture.value = null
}

watch(
  () => `${base.value.width}x${base.value.height}`,
  () => {
    view.value = { x: 0, y: 0, w: base.value.width, h: base.value.height }
  },
  { immediate: true },
)

function onWheel(e: WheelEvent) {
  e.preventDefault()
  const p = clientToSvg(e)
  if (!p) return
  zoomAt(p.x, p.y, e.deltaY > 0 ? 1.12 : 0.88)
}

function onSvgPointerDown(e: PointerEvent) {
  if (e.button !== 0) return
  svgRef.value?.setPointerCapture(e.pointerId)
  gesture.value = {
    kind: 'pan',
    pointerId: e.pointerId,
    cx: e.clientX,
    cy: e.clientY,
    vx: view.value.x,
    vy: view.value.y,
  }
}

function onNodePointerDown(e: PointerEvent, stem: string) {
  if (e.button !== 0) return
  e.stopPropagation()
  const pos = pointOf(stem)
  const p = clientToSvg(e)
  if (!pos || !p) return
  gesture.value = {
    kind: 'node',
    pointerId: e.pointerId,
    stem,
    sx: p.x,
    sy: p.y,
    ox: pos.x,
    oy: pos.y,
    moved: false,
  }
  svgRef.value?.setPointerCapture(e.pointerId)
}

function onPointerMove(e: PointerEvent) {
  const g = gesture.value
  if (!g || g.pointerId !== e.pointerId) return
  if (g.kind === 'node') {
    const p = clientToSvg(e)
    if (!p) return
    const dx = p.x - g.sx
    const dy = p.y - g.sy
    if (Math.hypot(dx, dy) > DRAG_PX) g.moved = true
    offsets.value = { ...offsets.value, [g.stem]: { x: g.ox + dx, y: g.oy + dy } }
    return
  }
  const rect = svgRef.value?.getBoundingClientRect()
  if (!rect?.width || !rect.height) return
  view.value = {
    ...view.value,
    x: g.vx - (e.clientX - g.cx) * (view.value.w / rect.width),
    y: g.vy - (e.clientY - g.cy) * (view.value.h / rect.height),
  }
}

function onPointerUp(e: PointerEvent) {
  const g = gesture.value
  if (!g || g.pointerId !== e.pointerId) return
  gesture.value = null
  if (g.kind === 'node' && !g.moved) openStem(g.stem)
}

watch(svgRef, (el, prev) => {
  prev?.removeEventListener('wheel', onWheel)
  el?.addEventListener('wheel', onWheel, { passive: false })
})

onBeforeUnmount(() => {
  svgRef.value?.removeEventListener('wheel', onWheel)
})
</script>

<template>
  <div class="novel-rel">
    <p v-if="!cards.length" class="novel-rel__empty">{{ t('novelWorkbench.noCastYet') }}</p>
    <template v-else>
      <p v-if="scene.truncated" class="novel-rel__note">{{ t('novelWorkbench.relationTruncated', { n: MAX_NODES }) }}</p>
      <div class="novel-rel__stage">
        <div class="novel-rel__tools">
          <button type="button" class="novel-rel__tool" @click="zoomCenter(0.8)">{{ t('novelWorkbench.relationZoomIn') }}</button>
          <button type="button" class="novel-rel__tool" @click="zoomCenter(1.25)">{{ t('novelWorkbench.relationZoomOut') }}</button>
          <button type="button" class="novel-rel__tool" @click="resetView">{{ t('novelWorkbench.relationReset') }}</button>
        </div>
        <svg
          ref="svgRef"
          class="novel-rel__svg"
          :class="{ 'novel-rel__svg--panning': gesture?.kind === 'pan' }"
          :viewBox="viewBox"
          role="img"
          :aria-label="t('novelWorkbench.castGraph')"
          @pointerdown="onSvgPointerDown"
          @pointermove="onPointerMove"
          @pointerup="onPointerUp"
          @pointercancel="onPointerUp"
        >
          <path
            v-for="line in scene.lines"
            :key="line.key"
            :d="line.d"
            class="novel-rel__edge"
            :class="{
              'novel-rel__edge--gap': line.missingBackEdge,
              'novel-rel__edge--unknown': line.unknownTarget,
            }"
          >
            <title>{{ line.fromLabel }} → {{ line.toLabel }}{{ line.current ? ' · ' + line.current : '' }}</title>
          </path>
          <g
            v-for="node in scene.positions"
            :key="node.stem"
            class="novel-rel__node"
            :class="nodeClass(node.role)"
            @pointerdown="onNodePointerDown($event, node.stem)"
          >
            <title>{{ node.title }}</title>
            <circle :cx="node.x" :cy="node.y" r="14" class="novel-rel__hit" />
            <circle :cx="node.x" :cy="node.y" r="7" />
            <text :x="node.tx" :y="node.ty" :text-anchor="node.anchor">{{ node.label }}</text>
          </g>
        </svg>
      </div>
      <p class="novel-rel__legend">
        <span class="novel-rel__swatch novel-rel__swatch--solid" /> {{ t('novelWorkbench.relationOk') }}
        <span class="novel-rel__swatch novel-rel__swatch--gap" /> {{ t('novelWorkbench.relationMissing') }}
      </p>
      <p v-if="!scene.lines.length" class="novel-rel__empty">{{ t('novelWorkbench.noRelations') }}</p>
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

.novel-rel__stage {
  position: relative;
}

.novel-rel__tools {
  position: absolute;
  top: 6px;
  right: 6px;
  z-index: 1;
  display: flex;
  gap: 4px;
}

.novel-rel__tool {
  padding: 2px 8px;
  border: 1px solid color-mix(in srgb, currentColor 25%, transparent);
  border-radius: 4px;
  background: color-mix(in srgb, var(--dq-bg, canvas) 88%, transparent);
  color: inherit;
  font: inherit;
  font-size: var(--dq-font-size-caption);
  cursor: pointer;
}

.novel-rel__svg {
  width: 100%;
  height: auto;
  max-height: 640px;
  touch-action: none;
  cursor: grab;
  user-select: none;
}

.novel-rel__svg--panning,
.novel-rel__svg--panning .novel-rel__node {
  cursor: grabbing;
}

.novel-rel__edge {
  fill: none;
  stroke: color-mix(in srgb, var(--dq-accent) 70%, currentColor);
  stroke-width: 1.35;
  stroke-linecap: round;
  pointer-events: none;
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
  cursor: grab;
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

.novel-rel__node circle.novel-rel__hit {
  fill: transparent;
  stroke: none;
}

.novel-rel__node text {
  fill: currentColor;
  font-size: 10px;
  pointer-events: none;
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
