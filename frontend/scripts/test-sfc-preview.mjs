import assert from 'node:assert/strict'
import { assembleSfcModule, SFC_VUE_GLOBAL } from '../src/utils/sfc-preview.ts'

// Script setup with an inlined template: keep `export default`, hoist vue imports.
const inline = assembleSfcModule({
  script: `import { ref as _ref, openBlock } from "vue"
export default { setup() { const n = _ref(0); return { n } } }`,
  inline: true,
})
assert.ok(inline.includes(`const { ref: _ref, openBlock } = ${SFC_VUE_GLOBAL};`), inline)
assert.ok(inline.includes('export default'), inline)
assert.ok(!inline.includes('from "vue"'), inline)

// Split mode: `export default` becomes a const so the compiled render attaches.
const split = assembleSfcModule({
  script: `import { defineComponent } from "vue"
export default defineComponent({ data: () => ({ n: 1 }) })`,
  template: `import { createElementVNode as _createElementVNode } from "vue"
export function render(_ctx) { return _createElementVNode("div", null, String(_ctx.n)) }`,
  inline: false,
})
assert.ok(split.includes('const __sfc_main = defineComponent'), split)
assert.ok(split.includes('function render(_ctx)'), split)
assert.ok(split.includes('export default Object.assign(__sfc_main, { render });'), split)
// Both chunks import from vue; bindings must be merged exactly once.
assert.equal(split.match(/const \{/g)?.length, 1, split)
assert.ok(split.includes('defineComponent') && split.includes('createElementVNode: _createElementVNode'), split)

// Template-only component still produces a renderable export.
const tplOnly = assembleSfcModule({
  template: `import { createElementVNode as _h } from "vue"
export function render(_ctx) { return _h("p", null, "hi") }`,
  inline: false,
})
assert.ok(tplOnly.includes('const __sfc_main = {};'), tplOnly)
assert.ok(tplOnly.includes('export default Object.assign(__sfc_main, { render });'), tplOnly)

console.log('sfc-preview ok')
