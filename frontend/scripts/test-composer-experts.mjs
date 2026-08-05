import assert from 'node:assert/strict'
import {
  buildExpertSummonPrefix,
  filterSummonableExperts,
  listSummonableExperts,
  prependExpertSummon,
} from '../src/types/composer-experts.ts'

const agents = [
  { id: 'team', name: 'Team', mode: 'primary', canDelegate: true },
  { id: 'document', name: 'Document', mode: 'subagent', description: 'Docs' },
  { id: 'github', name: 'GitHub', mode: 'subagent', description: 'Issues' },
  { id: 'novel', name: '小说创作', mode: 'subagent', description: 'Fiction' },
]

assert.deepEqual(
  listSummonableExperts(agents, 'team').map((a) => a.id).sort(),
  ['document', 'github', 'novel'],
)
assert.ok(!listSummonableExperts(agents, 'team').some((a) => a.id === 'team'))

const hits = filterSummonableExperts(agents, 'git', 'team')
assert.equal(hits.length, 1)
assert.equal(hits[0].id, 'github')

const prefix = buildExpertSummonPrefix(
  [agents[1]],
  (name, id) => `Delegate ${name} (${id})`,
  'Use delegate_agent.',
)
assert.ok(prefix.includes('Delegate Document (document)'))
assert.ok(prefix.includes('Use delegate_agent.'))

const out = prependExpertSummon(
  'Write a report',
  [agents[1]],
  (name, id) => `请委派「${name}」专家（agent_id=${id}）完成以下任务。`,
  '请使用 delegate_agent。',
)
assert.ok(out.startsWith('请委派「Document」'))
assert.ok(out.endsWith('Write a report'))

console.log('composer-experts helpers ok')
