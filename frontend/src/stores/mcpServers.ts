import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { ConnectorCatalogEntry, MCPServer, MCPToolDef } from '@/types'
import { fetchJSON, asArray } from '@/api/client'

export const useMcpServersStore = defineStore('mcpServers', () => {
  const items = ref<MCPServer[]>([])
  const catalog = ref<ConnectorCatalogEntry[]>([])

  async function load() {
    const data = await fetchJSON<MCPServer[]>('/mcp/servers')
    items.value = asArray(data)
  }

  async function loadCatalog() {
    const data = await fetchJSON<ConnectorCatalogEntry[]>('/mcp/catalog')
    catalog.value = asArray(data)
  }

  async function create(payload: Omit<MCPServer, 'id' | 'status'> & { headerSecrets?: Record<string, string> }) {
    const server = await fetchJSON<MCPServer>('/mcp/servers', {
      method: 'POST',
      body: JSON.stringify(payload),
    })
    items.value.push(server)
    return server
  }

  async function installCatalog(catalogId: string, name?: string) {
    const server = await fetchJSON<MCPServer>(`/mcp/catalog/${encodeURIComponent(catalogId)}/install`, {
      method: 'POST',
      body: JSON.stringify({ name }),
    })
    items.value.push(server)
    return server
  }

  async function update(id: string, payload: Partial<MCPServer> & { headerSecrets?: Record<string, string> }) {
    const server = await fetchJSON<MCPServer>(`/mcp/servers/${id}`, {
      method: 'PUT',
      body: JSON.stringify(payload),
    })
    const i = items.value.findIndex((s) => s.id === id)
    if (i >= 0) items.value[i] = server
    return server
  }

  async function remove(id: string) {
    await fetchJSON(`/mcp/servers/${id}`, { method: 'DELETE' })
    items.value = items.value.filter((s) => s.id !== id)
  }

  async function refreshTools(id: string): Promise<MCPToolDef[]> {
    const res = await fetchJSON<{ tools: MCPToolDef[] }>(`/mcp/servers/${id}/refresh-tools`, {
      method: 'POST',
    })
    await load()
    return res?.tools ?? []
  }

  async function toggleTool(id: string, toolName: string, enabled: boolean) {
    const server = await fetchJSON<MCPServer>(`/mcp/servers/${id}/tools/${encodeURIComponent(toolName)}`, {
      method: 'PATCH',
      body: JSON.stringify({ enabled }),
    })
    const i = items.value.findIndex((s) => s.id === id)
    if (i >= 0) items.value[i] = server
    return server
  }

  async function beginOAuth(id: string, redirectUri?: string) {
    return fetchJSON<{ authorizeUrl: string; state: string }>(`/mcp/servers/${id}/oauth/begin`, {
      method: 'POST',
      body: JSON.stringify({ redirectUri }),
    })
  }

  async function completeOAuth(id: string, payload: { code?: string; state?: string; accessToken?: string }) {
    const server = await fetchJSON<MCPServer>(`/mcp/servers/${id}/oauth/complete`, {
      method: 'POST',
      body: JSON.stringify(payload),
    })
    const i = items.value.findIndex((s) => s.id === id)
    if (i >= 0) items.value[i] = server
    return server
  }

  return {
    items,
    catalog,
    load,
    loadCatalog,
    create,
    installCatalog,
    update,
    remove,
    refreshTools,
    toggleTool,
    beginOAuth,
    completeOAuth,
  }
})
