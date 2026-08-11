import { defineStore } from 'pinia'
import { ref } from 'vue'
import { fetchJSON, asArray } from '@/api/client'
import type { Skill, SkillFile } from '@/types'

function skillFileURL(skillId: string, filePath: string) {
  const encoded = filePath
    .split('/')
    .filter(Boolean)
    .map(encodeURIComponent)
    .join('/')
  return `/skills/${skillId}/files/${encoded}`
}

export const useSkillsStore = defineStore('skills', () => {
  const items = ref<Skill[]>([])
  const loading = ref(false)

  async function load(projectIds?: string[]) {
    loading.value = true
    try {
      const global = asArray(await fetchJSON<Skill[]>('/skills'))
      items.value = global

      if (projectIds && projectIds.length > 0) {
        for (const pid of projectIds) {
          const projSkills = asArray(await fetchJSON<Skill[]>(`/skills?project_id=${encodeURIComponent(pid)}`))
          for (const s of projSkills) {
            if (!items.value.find((x) => x.id === s.id)) {
              items.value.push(s)
            }
          }
        }
      }
    } finally {
      loading.value = false
    }
  }

  async function get(skillId: string) {
    return fetchJSON<Skill>(`/skills/${skillId}`).catch(() => undefined)
  }

  async function create(payload: Omit<Skill, 'id'> & { id: string }) {
    const saved = await fetchJSON<Skill>('/skills', {
      method: 'POST',
      body: JSON.stringify(payload),
    })
    items.value.push(saved)
    return saved
  }

  async function update(skillId: string, payload: Skill) {
    const i = items.value.findIndex((s) => s.id === skillId)
    if (i < 0) throw new Error('Skill not found')
    const updated = await fetchJSON<Skill>(`/skills/${skillId}`, {
      method: 'PUT',
      body: JSON.stringify({ ...payload, id: skillId }),
    })
    items.value[i] = updated
    return updated
  }

  async function remove(skillId: string) {
    await fetchJSON(`/skills/${skillId}`, { method: 'DELETE' })
    items.value = items.value.filter((s) => s.id !== skillId)
  }

  async function importDir(dirPath: string) {
    const result = await fetchJSON<{ skill: Skill; fileCount: number }>('/skills/import', {
      method: 'POST',
      body: JSON.stringify({ path: dirPath }),
    })
    const i = items.value.findIndex((s) => s.id === result.skill.id)
    if (i >= 0) items.value[i] = result.skill
    else items.value.push(result.skill)
    return result
  }

  async function getFiles(skillId: string) {
    return asArray(await fetchJSON<SkillFile[]>(`/skills/${skillId}/files`).catch(() => [] as SkillFile[]))
  }

  async function getFileContent(skillId: string, filePath: string): Promise<string> {
    const resp = await fetch(`/api/v1${skillFileURL(skillId, filePath)}`)
    if (!resp.ok) throw new Error('File not found')
    return resp.text()
  }

  async function upsertFile(skillId: string, filePath: string, content: string) {
    return fetchJSON<SkillFile>(skillFileURL(skillId, filePath), {
      method: 'PUT',
      body: JSON.stringify({ content }),
    })
  }

  async function deleteFile(skillId: string, filePath: string) {
    await fetchJSON(skillFileURL(skillId, filePath), {
      method: 'DELETE',
    })
  }

  async function getExportMD(skillId: string) {
    const resp = await fetch(`/api/v1/skills/${skillId}/export`)
    if (!resp.ok) throw new Error('Export failed')
    return resp.text()
  }

  async function reset(skillId: string) {
    const updated = await fetchJSON<Skill>(`/skills/${skillId}/reset`, {
      method: 'POST',
    })
    const i = items.value.findIndex((s) => s.id === skillId)
    if (i >= 0) items.value[i] = updated
    return updated
  }

  async function refresh(skillId: string) {
    const updated = await get(skillId)
    if (!updated) return undefined
    const i = items.value.findIndex((s) => s.id === skillId)
    if (i >= 0) items.value[i] = updated
    else items.value.push(updated)
    return updated
  }

  return {
    items,
    loading,
    load,
    get,
    create,
    update,
    remove,
    importDir,
    getFiles,
    getFileContent,
    upsertFile,
    deleteFile,
    getExportMD,
    reset,
    refresh,
  }
})
