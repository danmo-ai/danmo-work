/**
 * Unit tests for open-session-tab semantics (open / close / prune / archive-remove).
 * Run: node --experimental-strip-types scripts/test-open-session-tabs.mjs
 */
import assert from 'node:assert/strict'

const COMPOSE_TAB_ID = '__compose__'

function createTabState(initial = []) {
  let openIds = [...initial]
  return {
    get openIds() {
      return openIds
    },
    open(id) {
      if (!id || id === COMPOSE_TAB_ID) return
      if (openIds.includes(id)) return
      openIds = [...openIds, id]
    },
    remove(id) {
      if (!openIds.includes(id)) return
      openIds = openIds.filter((x) => x !== id)
    },
    close(id) {
      const idx = openIds.indexOf(id)
      if (idx < 0) return null
      const next = openIds[idx + 1] ?? openIds[idx - 1] ?? null
      openIds = openIds.filter((x) => x !== id)
      return next
    },
    prune(validIds) {
      openIds = openIds.filter((id) => validIds.has(id))
    },
  }
}

// open adds once
{
  const s = createTabState()
  s.open('a')
  s.open('a')
  s.open('b')
  s.open(COMPOSE_TAB_ID)
  assert.deepEqual(s.openIds, ['a', 'b'])
}

// close returns adjacent next; does not imply archive
{
  const s = createTabState(['a', 'b', 'c'])
  assert.equal(s.close('b'), 'c')
  assert.deepEqual(s.openIds, ['a', 'c'])
  assert.equal(s.close('a'), 'c')
  assert.equal(s.close('c'), null)
  assert.deepEqual(s.openIds, [])
}

// archive/delete uses remove (no navigation hint)
{
  const s = createTabState(['a', 'b'])
  s.remove('a')
  assert.deepEqual(s.openIds, ['b'])
}

// prune drops missing sessions after reload
{
  const s = createTabState(['a', 'gone', 'b'])
  s.prune(new Set(['a', 'b']))
  assert.deepEqual(s.openIds, ['a', 'b'])
}

console.log('ok: open-session-tabs semantics')
