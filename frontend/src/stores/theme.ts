import { defineStore } from 'pinia'
import { ref, watch } from 'vue'
import {
  THEME_OPTIONS as DQ_THEME_OPTIONS,
  applyDqTheme,
  isDqThemeSlug,
  type DqThemeSlug,
} from '@danqing/dq-tokens'

export type ThemeId = DqThemeSlug

export interface ThemeOption {
  id: ThemeId
  label: string
  description: string
  htmlClass: string
  accent: string
  dark: boolean
}

/** Product Settings catalog — sourced from @danqing/dq-tokens. */
export const THEME_OPTIONS: ThemeOption[] = DQ_THEME_OPTIONS.map((opt) => ({
  id: opt.slug,
  label: opt.label,
  description: opt.description,
  htmlClass: opt.htmlClass,
  accent: opt.accent,
  dark: opt.dark,
}))

const STORAGE_KEY = 'dq-theme'

function applyTheme(id: ThemeId) {
  applyDqTheme(id)
}

function getStoredTheme(): ThemeId {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored && isDqThemeSlug(stored)) {
      return stored
    }
  } catch {
    // ignore
  }
  return 'tokyo-night'
}

export const useThemeStore = defineStore('theme', () => {
  const currentTheme = ref<ThemeId>(getStoredTheme())

  function setTheme(id: ThemeId) {
    currentTheme.value = id
    applyTheme(id)
    try {
      localStorage.setItem(STORAGE_KEY, id)
    } catch {
      // ignore
    }
  }

  function init() {
    applyTheme(currentTheme.value)
  }

  watch(currentTheme, (id) => {
    applyTheme(id)
    try {
      localStorage.setItem(STORAGE_KEY, id)
    } catch {
      // ignore
    }
  })

  return { currentTheme, setTheme, init }
})
