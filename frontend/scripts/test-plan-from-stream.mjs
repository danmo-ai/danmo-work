import assert from 'node:assert/strict'
import {
  mergePlanDocFromEvents,
  latestTodoWriteItems,
  buildPlanExecutePrefill,
  parsePlanTodos,
} from '../src/utils/plan-from-stream.ts'

assert.equal(parsePlanTodos(null).length, 0)
assert.deepEqual(
  parsePlanTodos([{ id: 't1', content: 'A', status: 'in_progress', priority: 'high' }]),
  [{ id: 't1', content: 'A', status: 'in_progress', priority: 'high' }],
)

const events = [
  {
    type: 'tool.running',
    payload: {
      name: 'create_plan',
      input: {
        name: 'Auth',
        overview: 'Add login',
        plan: '# Auth\n\n**Goal:** login',
        todos: [{ id: 't1', content: 'Wire route', status: 'pending' }],
      },
    },
  },
  {
    type: 'tool.completed',
    payload: {
      name: 'update_plan',
      input: {
        overview: 'Revised',
        todos: [
          { id: 't1', content: 'Wire route', status: 'completed' },
          { id: 't2', content: 'Tests', status: 'pending' },
        ],
      },
    },
  },
]

const doc = mergePlanDocFromEvents(events)
assert.ok(doc)
assert.equal(doc.name, 'Auth')
assert.equal(doc.overview, 'Revised')
assert.equal(doc.plan, '# Auth\n\n**Goal:** login')
assert.equal(doc.todos.length, 2)
assert.equal(doc.todos[0].status, 'completed')

const todoOnly = [
  {
    type: 'tool.running',
    turnId: 'turn-1',
    payload: {
      name: 'todowrite',
      input: {
        todos: [{ content: 'Legacy', status: 'pending', priority: 'medium' }],
      },
    },
  },
]
assert.equal(latestTodoWriteItems(todoOnly, 'turn-1')[0].content, 'Legacy')
assert.equal(latestTodoWriteItems(todoOnly, 'other').length, 0)

const prefill = buildPlanExecutePrefill(doc, doc.todos, {
  intro: 'Implement now.',
  todosHeader: '## Todos',
})
assert.ok(prefill.includes('Implement now.'))
assert.ok(prefill.includes('# Auth'))
assert.ok(prefill.includes('**Goal:** login'))
assert.ok(!prefill.includes('## Todos')) // plan markdown present → skip todo dump

const todoPrefill = buildPlanExecutePrefill(null, doc.todos, {
  intro: 'Go.',
  todosHeader: '## Todos',
})
assert.ok(todoPrefill.includes('## Todos'))
assert.ok(todoPrefill.includes('[x] Wire route'))
assert.ok(todoPrefill.includes('[ ] Tests'))

console.log('plan-from-stream helpers ok')
