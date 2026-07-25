/**
 * Compile every locale string under NODE_ENV=production.
 * vue-i18n throws SyntaxError (code 10 = Invalid linked format) for bare `@`
 * in production; desktop/Tauri builds hit this path.
 *
 * Usage: npm run check-i18n
 */
import { createI18n } from 'vue-i18n'
import zhCN from '../src/i18n/locales/zh-CN.ts'
import en from '../src/i18n/locales/en.ts'

process.env.NODE_ENV = 'production'

function flatten(obj, prefix = '') {
  const out = []
  for (const [k, v] of Object.entries(obj)) {
    const key = prefix ? `${prefix}.${k}` : k
    if (typeof v === 'string') out.push([key, v])
    else if (v && typeof v === 'object') out.push(...flatten(v, key))
  }
  return out
}

function check(locale, messages) {
  const i18n = createI18n({ legacy: false, locale, messages: { [locale]: messages } })
  let failed = 0
  for (const [key] of flatten(messages)) {
    try {
      i18n.global.t(key)
    } catch (e) {
      failed++
      console.error(`FAIL ${locale}.${key}: ${e.name}: ${e.message}`)
    }
  }
  console.log(`${locale}: ${flatten(messages).length} keys, ${failed} failed`)
  return failed
}

const failed = check('zh-CN', zhCN) + check('en', en)
process.exit(failed ? 1 : 0)
