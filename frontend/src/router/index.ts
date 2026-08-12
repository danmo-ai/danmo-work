import { createRouter, createWebHistory, createWebHashHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import type { AppModule } from '@/types/app-module'

const modules: AppModule[] = ['sessions', 'workers', 'knowledge', 'skills', 'mcpServers', 'automations', 'usage', 'settings', 'plugins']

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    redirect: '/sessions',
  },
  {
    path: '/sessions/:id?',
    name: 'sessions',
    component: () => import('@/views/SessionsView.vue'),
    props: (route) => ({ sessionId: route.params.id || null }),
  },
  ...modules.filter((m) => m !== 'sessions').map((m) => ({
    path: `/${m}`,
    name: m,
    component: () => import('@/views/ModuleView.vue'),
    props: { module: m },
  })),
]

export const router = createRouter({
  history: __TAURI_BUILD__ ? createWebHashHistory(__ROUTER_BASE__) : createWebHistory(__ROUTER_BASE__),
  routes,
})
