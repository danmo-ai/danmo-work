import { defineStore } from 'pinia'
import { computed, ref, watch } from 'vue'
import { i18n } from '@/i18n'

export type LocaleId = 'zh-CN' | 'en'
export type LocalePreference = 'system' | LocaleId

const STORAGE_KEY = 'dq-locale'

const LOCALE_IDS: LocaleId[] = ['zh-CN', 'en']

export function mapNavigatorLanguage(lang: string | undefined | null): LocaleId {
  if (!lang) return 'zh-CN'
  const lower = lang.toLowerCase()
  if (lower === 'zh' || lower.startsWith('zh-')) return 'zh-CN'
  return 'en'
}

export function resolveLocale(pref: LocalePreference): LocaleId {
  if (pref !== 'system') return pref
  try {
    return mapNavigatorLanguage(navigator.language)
  } catch {
    return 'zh-CN'
  }
}

function isLocaleId(value: string): value is LocaleId {
  return (LOCALE_IDS as string[]).includes(value)
}

function isPreference(value: string): value is LocalePreference {
  return value === 'system' || isLocaleId(value)
}

function getStoredPreference(): LocalePreference {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored && isPreference(stored)) return stored
  } catch {
    // ignore
  }
  return 'system'
}

function applyLocale(id: LocaleId) {
  const loc = i18n.global.locale as unknown as { value: LocaleId } | LocaleId
  if (typeof loc === 'object' && loc && 'value' in loc) {
    loc.value = id
  } else {
    ;(i18n.global as { locale: LocaleId }).locale = id
  }
  try {
    document.documentElement.lang = id
  } catch {
    // ignore
  }
}

export const useLocaleStore = defineStore('locale', () => {
  const preference = ref<LocalePreference>(getStoredPreference())
  const currentLocale = computed(() => resolveLocale(preference.value))

  function setPreference(pref: LocalePreference) {
    preference.value = pref
    applyLocale(resolveLocale(pref))
    try {
      localStorage.setItem(STORAGE_KEY, pref)
    } catch {
      // ignore
    }
  }

  function init() {
    applyLocale(currentLocale.value)
  }

  watch(preference, (pref) => {
    applyLocale(resolveLocale(pref))
    try {
      localStorage.setItem(STORAGE_KEY, pref)
    } catch {
      // ignore
    }
  })

  return { preference, currentLocale, setPreference, init }
})

/** Active UI locale string for localeCompare / Intl. */
export function uiLocale(): string {
  const loc = i18n.global.locale as unknown as { value: string } | string
  return typeof loc === 'string' ? loc : String(loc.value)
}
