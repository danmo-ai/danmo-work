import { defineStore } from 'pinia'
import { ref } from 'vue'
import { fetchJSON, asArray } from '@/api/client'
import { toast } from '@/utils/feedback'
import type { ModelConfig } from '@/types/mission'

export const useModelConfigStore = defineStore('modelConfig', () => {
  const models = ref<ModelConfig[]>([])
  const loading = ref(false)
  const saving = ref(false)

  async function load() {
    loading.value = true
    try {
      const data = await fetchJSON<ModelConfig[]>('/model-configs')
      models.value = asArray(data)
      // Old backends / stale catalogs: force built-in dialect overlay once.
      const missingDialect = models.value.some((m) => !m.reasoning_dialect)
      if (missingDialect) {
        try {
          const refreshed = await fetchJSON<ModelConfig[]>('/model-configs/refresh', { method: 'POST' })
          models.value = asArray(refreshed)
        } catch {
          /* refresh endpoint may be missing on older sidecars */
        }
      }
    } catch {
      models.value = []
    } finally {
      loading.value = false
    }
  }

  async function save(all: ModelConfig[]) {
    saving.value = true
    try {
      const data = await fetchJSON<ModelConfig[]>('/model-configs', {
        method: 'PUT',
        body: JSON.stringify(all),
      })
      models.value = asArray(data)
      toast.success('模型参数已保存')
    } catch (e) {
      toast.error(e instanceof Error ? e.message : '保存失败')
      throw e
    } finally {
      saving.value = false
    }
  }

  async function refreshFromBuiltin() {
    saving.value = true
    try {
      const data = await fetchJSON<ModelConfig[]>('/model-configs/refresh', {
        method: 'POST',
      })
      models.value = asArray(data)
      toast.success('模型参数已从内置目录刷新')
      return models.value
    } catch (e) {
      toast.error(e instanceof Error ? e.message : '刷新失败')
      throw e
    } finally {
      saving.value = false
    }
  }

  return { models, loading, saving, load, save, refreshFromBuiltin }
})
