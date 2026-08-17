import { defineStore } from 'pinia'
import { ref } from 'vue'
import { fetchJSON, asArray } from '@/api/client'
import { toast } from '@/utils/feedback'
import { i18n } from '@/i18n'
import type { ModelConfig } from '@/types/mission'

interface ModelConfigsResponse {
  models: ModelConfig[]
  catalogDiverged: boolean
}

function parseModelConfigsResponse(data: unknown): ModelConfigsResponse {
  // Legacy backends returned a bare array.
  if (Array.isArray(data)) {
    return { models: asArray<ModelConfig>(data), catalogDiverged: false }
  }
  const obj = data as ModelConfigsResponse | null
  return {
    models: asArray(obj?.models),
    catalogDiverged: !!obj?.catalogDiverged,
  }
}

export const useModelConfigStore = defineStore('modelConfig', () => {
  const models = ref<ModelConfig[]>([])
  const catalogDiverged = ref(false)
  const loading = ref(false)
  const saving = ref(false)

  async function load() {
    loading.value = true
    try {
      const data = await fetchJSON<ModelConfigsResponse | ModelConfig[]>('/model-configs')
      const parsed = parseModelConfigsResponse(data)
      models.value = parsed.models
      catalogDiverged.value = parsed.catalogDiverged
    } catch {
      models.value = []
      catalogDiverged.value = false
    } finally {
      loading.value = false
    }
  }

  async function save(all: ModelConfig[]) {
    saving.value = true
    try {
      const data = await fetchJSON<ModelConfigsResponse | ModelConfig[]>('/model-configs', {
        method: 'PUT',
        body: JSON.stringify(all),
      })
      const parsed = parseModelConfigsResponse(data)
      models.value = parsed.models
      catalogDiverged.value = parsed.catalogDiverged
      toast.success(i18n.global.t('settings.modelConfigSaved'))
    } catch (e) {
      toast.error(e instanceof Error ? e.message : i18n.global.t('common.saveFailed'))
      throw e
    } finally {
      saving.value = false
    }
  }

  async function refreshFromBuiltin() {
    saving.value = true
    try {
      const data = await fetchJSON<ModelConfigsResponse | ModelConfig[]>('/model-configs/refresh', {
        method: 'POST',
      })
      const parsed = parseModelConfigsResponse(data)
      models.value = parsed.models
      catalogDiverged.value = false
      toast.success(i18n.global.t('settings.modelConfigReset'))
      return models.value
    } catch (e) {
      toast.error(e instanceof Error ? e.message : i18n.global.t('skills.resetFailed'))
      throw e
    } finally {
      saving.value = false
    }
  }

  return { models, catalogDiverged, loading, saving, load, save, refreshFromBuiltin }
})
