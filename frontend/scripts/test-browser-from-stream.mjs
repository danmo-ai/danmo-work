import assert from 'node:assert/strict'
import { isBrowserActivityEvent, isBrowserCloseEvent } from '../src/utils/browser-from-stream.ts'
import { stageUri, stageFromUri } from '../src/utils/stage-uri.ts'

const event = (type, name) => ({ type, payload: { name } })
assert.equal(isBrowserActivityEvent(event('tool.running', 'browser_navigate')), true)
assert.equal(isBrowserActivityEvent(event('tool.running', 'browser_act')), true)
assert.equal(isBrowserActivityEvent(event('tool.completed', 'browser_act')), false)
assert.equal(isBrowserActivityEvent(event('tool.running', 'web_fetch')), false)
assert.equal(isBrowserCloseEvent(event('tool.completed', 'browser_close')), true)

const browser = { kind: 'browser-live', path: '', mode: 'view', engine: 'screencast', sessionId: 'sess/1' }
const browserUri = stageUri(browser)
assert.equal(browserUri, 'browser://sess%2F1')
assert.deepEqual(stageFromUri(browserUri), browser)
// No legacy aliases: the scheme is `browser://` only.
assert.equal(stageFromUri('agent://sess%2F1'), null)
const desktopUri = stageUri({ kind: 'desktop', path: '', mode: 'view', engine: 'novnc', sessionId: 'proj-1' })
assert.equal(desktopUri, 'vnc://proj-1')
assert.equal(stageFromUri(desktopUri)?.kind, 'desktop')
const preview = { kind: 'web', path: '', mode: 'view', engine: 'iframe', url: 'https://example.com/?a=1&b=2' }
assert.equal(stageUri(preview), preview.url)
assert.deepEqual(stageFromUri(preview.url), preview)
// An external URL is its own address; the backend proxy URL never leaks out.
assert.equal(
  stageUri({ kind: 'web', path: '', mode: 'view', engine: 'iframe', url: 'https://a.test/x' }),
  'https://a.test/x',
)
// Empty web stage advertises nothing (the bar shows a placeholder instead).
assert.equal(stageUri({ kind: 'web', path: '', mode: 'view', engine: 'iframe' }), '')
// A .vue preview must keep its engine through the address bar, or it would
// fall back to the iframe web surface.
const vue = { kind: 'web', path: 'src/Widget.vue', mode: 'view', engine: 'vue-sfc' }
const vueUri = stageUri(vue)
assert.ok(vueUri.startsWith('file:///'), vueUri)
const vueBack = stageFromUri(vueUri)
assert.equal(vueBack?.kind, 'web')
assert.equal(vueBack?.engine, 'vue-sfc')
assert.equal(vueBack?.path, 'src/Widget.vue')
// There is exactly one form for project files; `project://` was a lossy alias.
assert.equal(
  stageUri({ kind: 'web', path: 'index.html', mode: 'view', engine: 'iframe' }),
  'file:///index.html?kind=web&mode=view&engine=iframe',
)
assert.equal(stageFromUri('project://index.html'), null)
// A media file must not degrade to an iframe.
const media = { kind: 'media', path: 'photo.png', mode: 'view', engine: 'image' }
assert.deepEqual(stageFromUri(stageUri(media)), media)
assert.equal(stageFromUri('https://') , null)
assert.equal(stageFromUri('file:///tmp/secret'), null)
assert.equal(stageFromUri('danmo://agent-browser/sess'), null)
console.log('browser stream + protocol-routed Stage URI tests passed')
