import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Automation } from '@/types'

const AUTO_KEY = 'danmo-work-automations'

function loadJSON<T>(key: string): T[] {
  try {
    const raw = localStorage.getItem(key)
    return raw ? (JSON.parse(raw) as T[]) : []
  } catch {
    return []
  }
}

function saveJSON<T>(key: string, items: T[]) {
  localStorage.setItem(key, JSON.stringify(items))
}

export const useAutomationsStore = defineStore('automations', () => {
  const items = ref<Automation[]>(loadJSON<Automation>(AUTO_KEY))

  function save() {
    saveJSON(AUTO_KEY, items.value)
  }

  function create(payload: Omit<Automation, 'id'>) {
    const automation: Automation = {
      ...payload,
      id: `auto-${Date.now()}`,
    }
    items.value.push(automation)
    save()
    return automation
  }

  function update(id: string, payload: Partial<Automation>) {
    const i = items.value.findIndex((a) => a.id === id)
    if (i < 0) return undefined
    items.value[i] = { ...items.value[i], ...payload }
    save()
    return items.value[i]
  }

  function remove(id: string) {
    items.value = items.value.filter((a) => a.id !== id)
    save()
  }

  function toggle(id: string) {
    const i = items.value.findIndex((a) => a.id === id)
    if (i < 0) return undefined
    items.value[i].enabled = !items.value[i].enabled
    save()
    return items.value[i]
  }

  return { items, create, update, remove, toggle }
})
