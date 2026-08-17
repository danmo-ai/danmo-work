import { i18n } from '@/i18n'

const SECOND = 1000
const MINUTE = 60 * SECOND
const HOUR = 60 * MINUTE
const DAY = 24 * HOUR
const MONTH = 30 * DAY
const YEAR = 365 * DAY

export function formatRelativeTime(date: string | Date, now: Date = new Date()): string {
  const d = typeof date === 'string' ? new Date(date) : date
  const diff = now.getTime() - d.getTime()
  const t = i18n.global.t
  if (diff < 0) return t('time.future')
  if (diff < MINUTE) return t('time.justNow')
  if (diff < HOUR) return t('time.minutes', { n: Math.floor(diff / MINUTE) })
  if (diff < DAY) return t('time.hours', { n: Math.floor(diff / HOUR) })
  const days = Math.floor(diff / DAY)
  if (days < 30) return t('time.days', { n: days })
  const months = Math.floor(diff / MONTH)
  if (months < 12) return t('time.months', { n: months })
  return t('time.years', { n: Math.floor(diff / YEAR) })
}
