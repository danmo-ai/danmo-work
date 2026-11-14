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
assert.equal(browserUri, 'agent://sess%2F1')
assert.deepEqual(stageFromUri(browserUri), browser)
const desktopUri = stageUri({ kind: 'desktop', path: '', mode: 'view', engine: 'novnc', sessionId: 'proj-1' })
assert.equal(desktopUri, 'vnc://proj-1')
assert.equal(stageFromUri(desktopUri)?.kind, 'desktop')
const preview = { kind: 'web', path: '', mode: 'view', engine: 'iframe', url: 'https://example.com/?a=1&b=2' }
assert.equal(stageUri(preview), preview.url)
assert.deepEqual(stageFromUri(preview.url), preview)
assert.equal(stageFromUri('https://') , null)
assert.equal(stageFromUri('file:///tmp/secret'), null)
assert.equal(stageFromUri('danmo://agent-browser/sess'), null)
console.log('browser stream + protocol-routed Stage URI tests passed')
