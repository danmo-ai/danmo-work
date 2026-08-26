/** Danmo envelope around Univer official snapshots (IWorkbookData / IDocumentData / ISlideData). */

export type UniverIrFormat = 'univer-sheet' | 'univer-doc' | 'univer-slides'

export interface DanmoUniverEnvelope<T = unknown> {
  danmo: { format: UniverIrFormat; version: 1 }
  snapshot: T
}

export function isDanmoUniverEnvelope(value: unknown): value is DanmoUniverEnvelope {
  if (!value || typeof value !== 'object') return false
  const v = value as Record<string, unknown>
  const danmo = v.danmo
  if (!danmo || typeof danmo !== 'object') return false
  const d = danmo as Record<string, unknown>
  return (
    (d.format === 'univer-sheet' || d.format === 'univer-doc' || d.format === 'univer-slides') &&
    d.version === 1 &&
    'snapshot' in v
  )
}

export function wrapUniverSnapshot<T>(format: UniverIrFormat, snapshot: T): DanmoUniverEnvelope<T> {
  return { danmo: { format, version: 1 }, snapshot }
}

export function parseUniverFile<T = unknown>(
  text: string,
  expected?: UniverIrFormat,
): { format: UniverIrFormat | null; snapshot: T } {
  const raw = JSON.parse(text || '{}') as unknown
  if (isDanmoUniverEnvelope(raw)) {
    if (expected && raw.danmo.format !== expected) {
      throw new Error(`expected ${expected}, got ${raw.danmo.format}`)
    }
    return { format: raw.danmo.format, snapshot: raw.snapshot as T }
  }
  // Bare snapshot (no envelope).
  return { format: expected ?? null, snapshot: raw as T }
}

export function stringifyUniverFile<T>(format: UniverIrFormat, snapshot: T): string {
  return JSON.stringify(wrapUniverSnapshot(format, snapshot), null, 2) + '\n'
}

/** Sibling IR path for an MS Office or legacy file. */
export function siblingUniverIrPath(path: string, format: UniverIrFormat): string {
  const normalized = path.replace(/\\/g, '/')
  const i = normalized.lastIndexOf('.')
  const stem = i > 0 ? normalized.slice(0, i) : normalized
  const ext =
    format === 'univer-sheet' ? '.usheet.json' : format === 'univer-doc' ? '.udoc.json' : '.uslides.json'
  return `${stem}${ext}`
}
