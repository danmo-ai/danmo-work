/** Parse create_plan / update_plan (+ todowrite fallback) from session stream events. */

export interface PlanTodoItem {
  id?: string
  content: string
  status: 'pending' | 'in_progress' | 'completed' | 'cancelled'
  priority?: 'high' | 'medium' | 'low'
}

export interface PlanDoc {
  name: string
  overview: string
  plan: string
  todos: PlanTodoItem[]
}

export interface PlanStreamEvent {
  type: string
  turnId?: string
  payload?: unknown
}

const PLAN_TOOL_NAMES = new Set(['create_plan', 'update_plan'])

function asRecord(v: unknown): Record<string, unknown> | null {
  return v && typeof v === 'object' && !Array.isArray(v) ? (v as Record<string, unknown>) : null
}

export function parsePlanTodos(raw: unknown): PlanTodoItem[] {
  if (!Array.isArray(raw) || raw.length === 0) return []
  const out: PlanTodoItem[] = []
  for (const item of raw) {
    const m = asRecord(item)
    if (!m) continue
    const content = String(m.content ?? '').trim()
    if (!content) continue
    const statusRaw = String(m.status ?? 'pending')
    const status: PlanTodoItem['status'] =
      statusRaw === 'completed' || statusRaw === 'in_progress' || statusRaw === 'cancelled'
        ? statusRaw
        : 'pending'
    const priorityRaw = String(m.priority ?? 'medium')
    const priority: PlanTodoItem['priority'] =
      priorityRaw === 'high' || priorityRaw === 'low' ? priorityRaw : 'medium'
    const id = String(m.id ?? '').trim()
    out.push({ id: id || undefined, content, status, priority })
  }
  return out
}

/** Merge create_plan / update_plan tool payloads across the session stream. */
export function mergePlanDocFromEvents(events: PlanStreamEvent[]): PlanDoc | null {
  let doc: PlanDoc | null = null
  for (const ev of events) {
    if (ev.type !== 'tool.running' && ev.type !== 'tool.completed') continue
    const p = asRecord(ev.payload)
    const name = String(p?.name ?? '')
    if (!PLAN_TOOL_NAMES.has(name)) continue
    const input = asRecord(p?.input)
    if (!input) continue
    const nextName = String(input.name ?? '').trim()
    const nextOverview = String(input.overview ?? '').trim()
    const nextPlan = String(input.plan ?? '').trim()
    const nextTodos = parsePlanTodos(input.todos)
    if (name === 'create_plan') {
      if (!nextName && !nextPlan) continue
      doc = {
        name: nextName,
        overview: nextOverview,
        plan: nextPlan,
        todos: nextTodos,
      }
      continue
    }
    if (!doc) {
      if (!nextName && !nextOverview && !nextPlan && !nextTodos.length) continue
      doc = { name: nextName, overview: nextOverview, plan: nextPlan, todos: nextTodos }
      continue
    }
    if (nextName) doc = { ...doc, name: nextName }
    if (nextOverview) doc = { ...doc, overview: nextOverview }
    if (nextPlan) doc = { ...doc, plan: nextPlan }
    if (nextTodos.length) doc = { ...doc, todos: nextTodos }
  }
  return doc
}

/** Latest todowrite todos for a turn (legacy Plan panel progress). */
export function latestTodoWriteItems(
  events: PlanStreamEvent[],
  planTurnId?: string | null,
): PlanTodoItem[] {
  for (let i = events.length - 1; i >= 0; i--) {
    const ev = events[i]
    if (planTurnId && ev.turnId !== planTurnId) continue
    if (ev.type !== 'tool.running') continue
    const p = asRecord(ev.payload)
    if (p?.name !== 'todowrite') continue
    const input = asRecord(p?.input)
    if (!input?.todos) continue
    const items = parsePlanTodos(input.todos)
    if (items.length) return items
  }
  return []
}

export interface PlanProgress {
  completed: number
  total: number
}

function countTodoProgress(items: PlanTodoItem[]): PlanProgress {
  const total = items.length
  const completed = items.filter((item) => item.status === 'completed').length
  return { completed, total }
}

/**
 * Badge progress for the Plan rail: prefer newest create_plan/update_plan todos
 * session-wide, else todowrite on the active plan turn.
 */
export function planProgressFromEvents(
  events: PlanStreamEvent[],
  planTurnId?: string | null,
): PlanProgress {
  for (let i = events.length - 1; i >= 0; i--) {
    const ev = events[i]
    if (ev.type !== 'tool.running' && ev.type !== 'tool.completed') continue
    const p = asRecord(ev.payload)
    const name = String(p?.name ?? '')
    if (!PLAN_TOOL_NAMES.has(name)) continue
    const input = asRecord(p?.input)
    const items = parsePlanTodos(input?.todos)
    if (items.length) return countTodoProgress(items)
  }
  const legacy = latestTodoWriteItems(events, planTurnId)
  if (legacy.length) return countTodoProgress(legacy)
  return { completed: 0, total: 0 }
}

export function buildPlanExecutePrefill(
  doc: PlanDoc | null,
  todos: PlanTodoItem[],
  copy: { intro: string; todosHeader: string },
): string {
  const parts: string[] = []
  parts.push(copy.intro)
  parts.push('')
  if (doc?.name) parts.push(`# ${doc.name}`)
  if (doc?.overview) parts.push(`\n${doc.overview}\n`)
  if (doc?.plan?.trim()) {
    parts.push(doc.plan.trim())
  } else if (todos.length) {
    parts.push(copy.todosHeader)
    for (const item of todos) {
      if (item.status === 'cancelled') continue
      const mark = item.status === 'completed' ? '[x]' : '[ ]'
      parts.push(`- ${mark} ${item.content}`)
    }
  }
  return parts.join('\n').trim()
}
