/**
 * Lightweight unit test for nested wheel → session scroll redirect.
 * Run: node --experimental-strip-types scripts/test-nested-scroll-chain.mjs
 */
import { nestedWheelRedirectDelta } from '../src/utils/nested-scroll-chain.ts'

// Node has no DOM Element — provide minimal stubs for instanceof checks.
class Element {}
class HTMLElement extends Element {}
globalThis.Element = Element
globalThis.HTMLElement = HTMLElement


function style(el, overrides) {
  Object.defineProperty(el, 'ownerDocument', { value: { defaultView: globalThis } })
  el._style = { overflowX: 'visible', overflowY: 'visible', ...overrides }
}

// Minimal DOM stubs for getComputedStyle / Element traversal.
globalThis.getComputedStyle = (el) => el._style || { overflowX: 'visible', overflowY: 'visible' }

function el(props = {}) {
  const node = new HTMLElement()
  node.scrollTop = props.scrollTop ?? 0
  node.scrollHeight = props.scrollHeight ?? 100
  node.clientHeight = props.clientHeight ?? 100
  node.scrollWidth = props.scrollWidth ?? 100
  node.clientWidth = props.clientWidth ?? 100
  node.parentElement = null
  node._style = {
    overflowX: props.overflowX ?? 'visible',
    overflowY: props.overflowY ?? 'visible',
  }
  return node
}

let failed = 0
function expect(name, actual, wanted) {
  const ok = Object.is(actual, wanted)
  if (!ok) {
    failed++
    console.error(`FAIL ${name}: got ${actual}, want ${wanted}`)
  } else {
    console.log(`ok   ${name}`)
  }
}

const root = el({ scrollHeight: 2000, clientHeight: 400 })

// Nested vertical scroller mid-way: keep default (null).
{
  const nested = el({ overflowY: 'auto', scrollTop: 40, scrollHeight: 400, clientHeight: 100 })
  nested.parentElement = root
  expect('nested mid keeps default', nestedWheelRedirectDelta(root, nested, 0, 30), null)
}

// Nested vertical scroller at bottom + scroll down: redirect.
{
  const nested = el({ overflowY: 'auto', scrollTop: 300, scrollHeight: 400, clientHeight: 100 })
  nested.parentElement = root
  expect('nested at bottom redirects', nestedWheelRedirectDelta(root, nested, 0, 40), 40)
}

// Nested vertical scroller at top + scroll up: redirect.
{
  const nested = el({ overflowY: 'auto', scrollTop: 0, scrollHeight: 400, clientHeight: 100 })
  nested.parentElement = root
  expect('nested at top redirects up', nestedWheelRedirectDelta(root, nested, 0, -40), -40)
}

// overflow-x only (code fence): vertical delta redirects.
{
  const pre = el({ overflowX: 'auto', scrollWidth: 800, clientWidth: 400 })
  pre.parentElement = root
  expect('overflow-x traps vertical → redirect', nestedWheelRedirectDelta(root, pre, 0, 25), 25)
}

// Mostly horizontal gesture: ignore.
{
  const pre = el({ overflowX: 'auto', scrollWidth: 800, clientWidth: 400 })
  pre.parentElement = root
  expect('horizontal gesture ignored', nestedWheelRedirectDelta(root, pre, 40, 10), null)
}

// No nested scrollport: no redirect (browser scrolls root naturally).
{
  const plain = el()
  plain.parentElement = root
  expect('plain child no redirect', nestedWheelRedirectDelta(root, plain, 0, 20), null)
}

if (failed) {
  console.error(`\n${failed} failure(s)`)
  process.exit(1)
}
console.log('\nAll nested-scroll-chain checks passed.')
