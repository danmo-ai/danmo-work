/** True when the element can scroll vertically (overflow + content taller than box). */
export function canScrollVertically(el: HTMLElement): boolean {
  const { overflowY } = getComputedStyle(el)
  if (overflowY !== 'auto' && overflowY !== 'scroll' && overflowY !== 'overlay') return false
  return el.scrollHeight > el.clientHeight + 1
}

/** True when the element can scroll horizontally. */
export function canScrollHorizontally(el: HTMLElement): boolean {
  const { overflowX } = getComputedStyle(el)
  if (overflowX !== 'auto' && overflowX !== 'scroll' && overflowX !== 'overlay') return false
  return el.scrollWidth > el.clientWidth + 1
}

/**
 * Decide whether a wheel event over a nested target should be redirected to `root`.
 *
 * Chat transcripts often nest `overflow-y: auto` (thinking / tool output) and
 * `overflow-x: auto` (code fences). Those scrollports trap trackpad/wheel deltas
 * even at their vertical boundary — or when they only scroll on X — so the
 * session page feels "stuck" until the cursor leaves the nested region.
 *
 * Returns the deltaY to apply on `root`, or `null` if the browser should keep
 * the default target.
 */
export function nestedWheelRedirectDelta(
  root: HTMLElement,
  target: EventTarget | null,
  deltaX: number,
  deltaY: number,
): number | null {
  if (!deltaY || Math.abs(deltaY) < Math.abs(deltaX)) return null

  const isElement = (v: unknown): v is Element =>
    typeof Element !== 'undefined' && v instanceof Element
  const isHtml = (v: unknown): v is HTMLElement =>
    typeof HTMLElement !== 'undefined' && v instanceof HTMLElement

  let node: HTMLElement | null = isElement(target) ? (target as HTMLElement) : null
  while (node && node !== root) {
    if (isHtml(node)) {
      if (canScrollVertically(node)) {
        const atTop = node.scrollTop <= 0
        const atBottom = node.scrollTop + node.clientHeight >= node.scrollHeight - 1
        if ((deltaY < 0 && atTop) || (deltaY > 0 && atBottom)) return deltaY
        return null
      }

      // Chrome/Safari: overflow-x scrollports trap vertical trackpad deltas even
      // when there is no vertical overflow (and sometimes with no X overflow yet).
      const { overflowX } = getComputedStyle(node)
      if (overflowX === 'auto' || overflowX === 'scroll' || overflowX === 'overlay') {
        if (!canScrollVertically(node)) return deltaY
      }
    }
    node = node.parentElement
  }
  return null
}
