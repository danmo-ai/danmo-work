import { nextTick, type Ref } from 'vue'

export function isEditableKeyboardTarget(target: EventTarget | null) {
  return target instanceof HTMLElement && !!target.closest('input, textarea, select, [contenteditable="true"]')
}

export function scrollActiveRailRowIntoView() {
  nextTick(() => {
    document.querySelector('.resource-rail__row.is-active')?.scrollIntoView({ block: 'nearest' })
  })
}

export function handleResourceRailArrowKeys(
  e: KeyboardEvent,
  items: { id: string }[],
  selectedId: Ref<string | null>,
  onSelect: (id: string) => void,
  enabled = true,
): boolean {
  if (!enabled || isEditableKeyboardTarget(e.target)) return false
  if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return false
  if (!items.length) return false

  e.preventDefault()
  const idx = items.findIndex((item) => item.id === selectedId.value)
  const nextIdx =
    e.key === 'ArrowDown'
      ? idx < 0
        ? 0
        : Math.min(idx + 1, items.length - 1)
      : idx < 0
        ? items.length - 1
        : Math.max(idx - 1, 0)
  onSelect(items[nextIdx].id)
  scrollActiveRailRowIntoView()
  return true
}

export function handleResourceRailArrowKeysByKey(
  e: KeyboardEvent,
  items: { key: string }[],
  selectedKey: Ref<string | null>,
  onSelect: (key: string) => void,
  enabled = true,
): boolean {
  if (!enabled || isEditableKeyboardTarget(e.target)) return false
  if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return false
  if (!items.length) return false

  e.preventDefault()
  const idx = items.findIndex((item) => item.key === selectedKey.value)
  const nextIdx =
    e.key === 'ArrowDown'
      ? idx < 0
        ? 0
        : Math.min(idx + 1, items.length - 1)
      : idx < 0
        ? items.length - 1
        : Math.max(idx - 1, 0)
  onSelect(items[nextIdx].key)
  scrollActiveRailRowIntoView()
  return true
}
