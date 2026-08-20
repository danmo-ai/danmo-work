import assert from 'node:assert/strict'
import {
  buildExpertSummonPrefix,
  filterSummonableExperts,
  groupSummonableExperts,
  listSummonableExperts,
  normalizeExpertCategory,
  prependExpertSummon,
} from '../src/types/composer-experts.ts'

const agents = [
  { id: 'team', name: 'Team', mode: 'primary', canDelegate: true },
  { id: 'document', name: 'Document', mode: 'subagent', description: 'Docs', category: 'office' },
  { id: 'github', name: 'GitHub', mode: 'subagent', description: 'Issues', category: 'coding' },
  { id: 'novel', name: '小说创作', mode: 'subagent', description: 'Fiction', category: 'creative' },
  { id: 'browser', name: 'Browser', mode: 'subagent', description: 'Web UI', category: 'research' },
]

assert.deepEqual(
  listSummonableExperts(agents, 'team').map((a) => a.id).sort(),
  ['browser', 'document', 'github', 'novel'],
)
assert.ok(!listSummonableExperts(agents, 'team').some((a) => a.id === 'team'))

const hits = filterSummonableExperts(agents, 'git', 'team')
assert.equal(hits.length, 1)
assert.equal(hits[0].id, 'github')

assert.equal(normalizeExpertCategory('coding'), 'coding')
assert.equal(normalizeExpertCategory(''), 'other')

const groups = groupSummonableExperts(agents, '', 'team')
assert.deepEqual(groups.map((g) => g.id), ['coding', 'research', 'office', 'creative'])
assert.deepEqual(groups.find((g) => g.id === 'coding')?.agents.map((a) => a.id), ['github'])
assert.deepEqual(groups.find((g) => g.id === 'office')?.agents.map((a) => a.id), ['document'])

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
