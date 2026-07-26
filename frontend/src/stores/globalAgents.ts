import { defineStore } from 'pinia'
import { ref } from 'vue'
import { fetchJSON, asArray, isNotFoundError } from '@/api/client'
import type { Agent, CreateAgentPayload, UpdateAgentPayload } from '@/types'

export const useGlobalAgentsStore = defineStore('globalAgents', () => {
  const items = ref<Agent[]>([])

  async function load() {
    try {
      items.value = asArray(await fetchJSON<Agent[]>('/agents'))
    } catch (e) {
      items.value = []
      if (!isNotFoundError(e)) throw e
    }
  }

  async function get(agentId: string) {
    return fetchJSON<Agent>(`/agents/${agentId}`).catch(() => undefined)
  }

  async function create(payload: CreateAgentPayload) {
    const agent: Agent = {
      ...payload,
      description: payload.description ?? '',
      mode: payload.mode ?? 'primary',
      steps: payload.steps ?? 0,
    }
    const saved = await fetchJSON<Agent>('/agents', {
      method: 'POST',
      body: JSON.stringify(agent),
    })
    items.value.push(saved)
    return saved
  }

  async function update(agentId: string, payload: UpdateAgentPayload) {
    const i = items.value.findIndex((a) => a.id === agentId)
    if (i < 0) throw new Error('Agent not found')
    const updated = await fetchJSON<Agent>(`/agents/${agentId}`, {
      method: 'PUT',
      body: JSON.stringify({ ...items.value[i], ...payload }),
    })
    items.value[i] = updated
    return updated
  }

  async function remove(agentId: string) {
    await fetchJSON(`/agents/${agentId}`, { method: 'DELETE' })
    items.value = items.value.filter((a) => a.id !== agentId)
  }

  async function reset(agentId: string) {
    const i = items.value.findIndex((a) => a.id === agentId)
    if (i < 0) throw new Error('Agent not found')
    const resetAgent = await fetchJSON<Agent>(`/agents/${agentId}/reset`, { method: 'POST' })
    items.value[i] = resetAgent
    return resetAgent
  }

  return { items, load, get, create, update, remove, reset }
})
