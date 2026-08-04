import { defineStore } from 'pinia'
import { ref } from 'vue'
import { asArray, fetchJSON } from '@/api/client'
import type { KnowledgeBase, KnowledgeDocument } from '@/types'

export const useKnowledgeStore = defineStore('knowledge', () => {
  const bases = ref<KnowledgeBase[]>([])
  const documents = ref<KnowledgeDocument[]>([])
  const loading = ref(false)

  async function loadBases() {
    loading.value = true
    try {
      bases.value = asArray(await fetchJSON<KnowledgeBase[]>('/knowledge/bases'))
    } finally {
      loading.value = false
    }
  }

  async function loadDocs(baseId: string) {
    const docs = asArray(await fetchJSON<KnowledgeDocument[]>(`/knowledge/bases/${baseId}/docs`))
    documents.value = [
      ...documents.value.filter((d) => d.knowledgeBaseId !== baseId),
      ...docs.map((d) => ({
        ...d,
        knowledgeBaseId: d.knowledgeBaseId || (d as { kbId?: string }).kbId || baseId,
      })),
    ]
    return documentsFor(baseId)
  }

  async function createBase(payload: { name: string; description?: string }) {
    const base = await fetchJSON<KnowledgeBase>('/knowledge/bases', {
      method: 'POST',
      body: JSON.stringify({
        name: payload.name,
        description: payload.description ?? '',
      }),
    })
    bases.value = [base, ...bases.value.filter((b) => b.id !== base.id)]
    return base
  }

  async function updateBase(id: string, payload: { name: string; description?: string }) {
    const updated = await fetchJSON<KnowledgeBase>(`/knowledge/bases/${id}`, {
      method: 'PUT',
      body: JSON.stringify({
        name: payload.name,
        description: payload.description ?? '',
      }),
    })
    const i = bases.value.findIndex((b) => b.id === id)
    if (i >= 0) bases.value[i] = updated
    else bases.value.unshift(updated)
    return updated
  }

  async function removeBase(id: string) {
    await fetchJSON(`/knowledge/bases/${id}`, { method: 'DELETE' })
    bases.value = bases.value.filter((b) => b.id !== id)
    documents.value = documents.value.filter((d) => d.knowledgeBaseId !== id)
  }

  async function addDocument(baseId: string, title: string, content: string) {
    const raw = await fetchJSON<KnowledgeDocument & { kbId?: string }>(`/knowledge/bases/${baseId}/docs`, {
      method: 'POST',
      body: JSON.stringify({ title, content }),
    })
    const doc: KnowledgeDocument = {
      id: raw.id,
      knowledgeBaseId: raw.knowledgeBaseId || raw.kbId || baseId,
      title: raw.title,
      content: raw.content,
      updatedAt: raw.updatedAt,
    }
    documents.value = [doc, ...documents.value.filter((d) => d.id !== doc.id)]
    const base = bases.value.find((b) => b.id === baseId)
    if (base) {
      base.documentCount = documents.value.filter((d) => d.knowledgeBaseId === baseId).length
      base.updatedAt = doc.updatedAt
    }
    return doc
  }

  async function updateDocument(docId: string, title: string, content: string) {
    const raw = await fetchJSON<KnowledgeDocument & { kbId?: string }>(`/knowledge/docs/${docId}`, {
      method: 'PUT',
      body: JSON.stringify({ title, content }),
    })
    const doc: KnowledgeDocument = {
      id: raw.id,
      knowledgeBaseId: raw.knowledgeBaseId || raw.kbId || '',
      title: raw.title,
      content: raw.content,
      updatedAt: raw.updatedAt,
    }
    const i = documents.value.findIndex((d) => d.id === docId)
    if (i >= 0) documents.value[i] = doc
    else documents.value.unshift(doc)
    return doc
  }

  async function removeDocument(docId: string) {
    const doc = documents.value.find((d) => d.id === docId)
    await fetchJSON(`/knowledge/docs/${docId}`, { method: 'DELETE' })
    documents.value = documents.value.filter((d) => d.id !== docId)
    if (doc) {
      const base = bases.value.find((b) => b.id === doc.knowledgeBaseId)
      if (base) {
        base.documentCount = documents.value.filter((d) => d.knowledgeBaseId === base.id).length
        base.updatedAt = new Date().toISOString()
      }
    }
  }

  async function getDocument(docId: string) {
    const raw = await fetchJSON<KnowledgeDocument & { kbId?: string }>(`/knowledge/docs/${docId}`)
    return {
      id: raw.id,
      knowledgeBaseId: raw.knowledgeBaseId || raw.kbId || '',
      title: raw.title,
      content: raw.content,
      updatedAt: raw.updatedAt,
    } satisfies KnowledgeDocument
  }

  function documentsFor(baseId: string) {
    return documents.value.filter((d) => d.knowledgeBaseId === baseId)
  }

  return {
    bases,
    documents,
    loading,
    loadBases,
    loadDocs,
    createBase,
    updateBase,
    removeBase,
    deleteBase: removeBase,
    addDocument,
    updateDocument,
    removeDocument,
    getDocument,
    documentsFor,
  }
})
