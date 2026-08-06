/** Extensible workbench ids hosted in the session split pane. */
export type WorkbenchId = 'novel'

export interface WorkbenchRegistryEntry {
  id: WorkbenchId
  /** i18n key under workbench.* */
  labelKey: string
}

export const WORKBENCH_REGISTRY: WorkbenchRegistryEntry[] = [
  { id: 'novel', labelKey: 'workbench.novel' },
]

export function isWorkbenchId(v: string): v is WorkbenchId {
  return WORKBENCH_REGISTRY.some((e) => e.id === v)
}
