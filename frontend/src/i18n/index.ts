import { createI18n } from 'vue-i18n'
import zhCN from './locales/zh-CN'
import en from './locales/en'

export type MessageSchema = typeof zhCN

export const i18n = createI18n<[MessageSchema], 'zh-CN' | 'en'>({
  legacy: false,
  locale: 'zh-CN',
  fallbackLocale: 'zh-CN',
  messages: {
    'zh-CN': zhCN,
    en,
  },
})

if (import.meta.hot) {
  import.meta.hot.accept('./locales/zh-CN', (mod) => {
    if (mod?.default) i18n.global.setLocaleMessage('zh-CN', mod.default)
  })
  import.meta.hot.accept('./locales/en', (mod) => {
    if (mod?.default) i18n.global.setLocaleMessage('en', mod.default)
  })
}
