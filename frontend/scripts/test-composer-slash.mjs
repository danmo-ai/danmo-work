import assert from 'node:assert/strict'
import {
  COMPOSER_SLASH_COMMANDS,
  detectSlashQuery,
  filterSlashCommands,
  removeSlashQuery,
} from '../src/types/composer-slash.ts'
import { activityRank } from '../src/types/session-activity.ts'

assert.deepEqual(detectSlashQuery('/ch', 3), { start: 0, query: 'ch' })
assert.deepEqual(detectSlashQuery('hi /plan', 8), { start: 3, query: 'plan' })
assert.equal(detectSlashQuery('a/b', 3), null)

const hits = filterSlashCommands(COMPOSER_SLASH_COMMANDS, 'diff')
assert.ok(hits.some((c) => c.id === 'changes'))

assert.equal(removeSlashQuery('hi /plan more', 3, 8), 'hi  more')
assert.ok(activityRank('awaiting_approval') < activityRank('running'))
assert.ok(activityRank('running') < activityRank('idle'))

console.log('composer-slash / session-activity helpers ok')
