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
  '',
)
assert.equal(prefix, 'Delegate Document (document)\n\n')
assert.equal(
  buildExpertSummonPrefix([agents[1]], (name, id) => `Delegate ${name} (${id})`, '  Use tool.  '),
  'Delegate Document (document)\nUse tool.\n\n',
)

const out = prependExpertSummon(
  'Write a report',
  [agents[1]],
  (name, id) => `请用 delegate_agent(agent_id=${id}) 委派「${name}」；goal 写清意图，勿代做。`,
  '',
)
assert.ok(out.startsWith('请用 delegate_agent(agent_id=document) 委派「Document」'))
assert.ok(out.endsWith('Write a report'))
assert.ok(!out.includes('召集上述专家'))

console.log('composer-experts helpers ok')
