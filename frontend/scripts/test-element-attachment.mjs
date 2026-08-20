import assert from 'node:assert/strict'
import {
  fromInspectPayload,
  serializeElementAttachment,
  serializeElementAttachments,
  serializeComputedStyles,
  serializePreviewConsoleReport,
} from '../src/types/element-attachment.ts'

const att = fromInspectPayload(
  {
    tag: 'button',
    text: '开始游戏',
    outerHTML: '<button class="start">开始游戏</button>',
    neighborhoodHTML: '<div>\n  <!-- selected -->\n  <button class="start">开始游戏</button>\n</div>',
    computedStyles: {
      display: 'inline-flex',
      'font-size': '16px',
      color: 'rgb(255, 255, 255)',
      'background-color': 'rgb(16, 185, 129)',
    },
    id: 'start-btn',
    classes: ['start'],
    role: 'button',
    ariaLabel: '开始游戏',
    name: '',
    placeholder: '',
    testId: 'start',
    selectors: {
      css: 'button[data-testid="start"]',
      fallbacks: ['button.start', 'div > button:nth-of-type(1)'],
    },
    xpath: '/html/body/div/button[1]',
    boundingBox: { x: 120, y: 80, w: 160, h: 44 },
    viewport: { w: 800, h: 600 },
    attributes: { type: 'button' },
    component: { name: 'StartButton', file: 'src/Start.vue', line: 12, framework: 'vue' },
    page: { url: 'http://localhost/catch.html', title: 'game' },
  },
  { annotation: '改成「开始」', sourceFile: 'catch-apples.html' },
)

const block = serializeElementAttachment(att)
assert.match(block, /## Selected UI Element/)
assert.match(block, /Request: 改成「开始」/)
assert.match(block, /File: catch-apples.html/)
assert.match(block, /Component: StartButton \(src\/Start\.vue:12\)/)
assert.match(block, /XPath: \/html\/body\/div\/button\[1\]/)
assert.match(block, /Selectors: button\[data-testid="start"\]/)
assert.match(block, /Attrs: role=button/)
assert.match(block, /data-testid=start/)
assert.match(block, /Box: 120,80 160×44/)
assert.match(block, /Computed CSS: /)
assert.match(block, /font-size: 16px/)
assert.match(block, /```html/)

const css = serializeComputedStyles({ display: 'flex', color: 'red', bogus: '1' })
assert.match(css, /display: flex/)
assert.match(css, /color: red/)
assert.match(css, /bogus: 1/)

const related = serializeElementAttachments([att, att])
assert.match(related, /related/)
assert.equal([...related.matchAll(/## Selected UI Element/g)].length, 2)

const report = serializePreviewConsoleReport(
  [{ kind: 'error', message: 'Uncaught TypeError', source: 'app.js:3' }],
  'fix the crash',
)
assert.match(report, /## Preview Console \/ Network/)
assert.match(report, /Request: fix the crash/)
assert.match(report, /\[error\] Uncaught TypeError/)

console.log('element-attachment serialize ok')
