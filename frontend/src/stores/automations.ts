import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Automation } from '@/types'
import { fetchJSON, asArray } from '@/api/client'

export const useAutomationsStore = defineStore('automations', () => {
  const items = ref<Automation[]>([])

  async function load() {
    const data = await fetchJSON<Automation[]>('/automations')
    items.value = asArray(data)
  }

  async function create(payload: Omit<Automation, 'id'>) {
    const automation = await fetchJSON<Automation>('/automations', {
      method: 'POST',
      body: JSON.stringify(payload),
    })
    items.value.unshift(automation)
    return automation
  }

  async function update(id: string, payload: Partial<Automation>) {
    const automation = await fetchJSON<Automation>(`/automations/${id}`, {
      method: 'PUT',
      body: JSON.stringify(payload),
    })
    const i = items.value.findIndex((a) => a.id === id)
    if (i >= 0) items.value[i] = automation
    return automation
  }

  async function remove(id: string) {
    await fetchJSON(`/automations/${id}`, { method: 'DELETE' })
    items.value = items.value.filter((a) => a.id !== id)
  }

  async function toggle(id: string) {
    const automation = await fetchJSON<Automation>(`/automations/${id}/toggle`, {
      method: 'POST',
    })
    const i = items.value.findIndex((a) => a.id === id)
    if (i >= 0) items.value[i] = automation
    return automation
  }

  async function run(id: string) {
    const automation = await fetchJSON<Automation>(`/automations/${id}/run`, {
      method: 'POST',
    })
    const i = items.value.findIndex((a) => a.id === id)
    if (i >= 0) items.value[i] = automation
    return automation
  }

  return { items, load, create, update, remove, toggle, run }
})
