/**
 * Minimal IWorkbookData builders (Univer Sheets snapshot shape).
 * Kept dependency-free so CSV / legacy migration works without loading Univer.
 */

export interface MinimalCellData {
  v?: string | number | boolean
  t?: number
  f?: string
}

export interface MinimalWorksheetData {
  id: string
  name: string
  rowCount: number
  columnCount: number
  mergeData: Array<{ startRow: number; endRow: number; startColumn: number; endColumn: number }>
  cellData: Record<string, Record<string, MinimalCellData>>
  rowData: Record<string, unknown>
  columnData: Record<string, { w?: number }>
  defaultColumnWidth: number
  defaultRowHeight: number
  showGridlines: number
  freeze: { startRow: number; startColumn: number; ySplit: number; xSplit: number }
  scrollTop: number
  scrollLeft: number
  zoomRatio: number
  hidden: number
  tabColor: string
  rightToLeft: number
}

export interface MinimalWorkbookData {
  id: string
  name: string
  appVersion: string
  locale: string
  styles: Record<string, unknown>
  sheetOrder: string[]
  sheets: Record<string, MinimalWorksheetData>
}

function newId(prefix: string): string {
  return `${prefix}_${Math.random().toString(36).slice(2, 10)}`
}

export function emptyWorkbookData(name = 'Workbook'): MinimalWorkbookData {
  const sheetId = newId('sheet')
  return {
    id: newId('workbook'),
    name,
    appVersion: '0.25.1',
    locale: 'enUS',
    styles: {},
    sheetOrder: [sheetId],
    sheets: {
      [sheetId]: emptyWorksheetData(sheetId, 'Sheet1'),
    },
  }
}

export function emptyWorksheetData(id: string, name: string): MinimalWorksheetData {
  return {
    id,
    name,
    rowCount: 100,
    columnCount: 26,
    mergeData: [],
    cellData: {},
    rowData: {},
    columnData: {},
    defaultColumnWidth: 88,
    defaultRowHeight: 24,
    showGridlines: 1,
    freeze: { startRow: -1, startColumn: -1, ySplit: 0, xSplit: 0 },
    scrollTop: 0,
    scrollLeft: 0,
    zoomRatio: 1,
    hidden: 0,
    tabColor: '',
    rightToLeft: 0,
  }
}

/** Convert rectangular string rows into Univer cellData. */
export function rowsToWorksheetData(
  name: string,
  rows: string[][],
  colWidths?: number[],
): MinimalWorksheetData {
  const sheetId = newId('sheet')
  const ws = emptyWorksheetData(sheetId, name)
  const rowCount = Math.max(1, rows.length)
  const colCount = Math.max(1, ...rows.map((r) => r.length), 1)
  ws.rowCount = Math.max(100, rowCount)
  ws.columnCount = Math.max(26, colCount)
  const cellData: MinimalWorksheetData['cellData'] = {}
  for (let r = 0; r < rows.length; r++) {
    const row = rows[r] || []
    for (let c = 0; c < row.length; c++) {
      const raw = row[c] ?? ''
      if (raw === '') continue
      if (!cellData[r]) cellData[r] = {}
      if (raw.trimStart().startsWith('=')) {
        cellData[r][c] = { f: raw, v: '', t: 2 }
      } else if (/^-?\d+(\.\d+)?$/.test(raw.trim())) {
        cellData[r][c] = { v: Number(raw.trim()), t: 2 }
      } else {
        cellData[r][c] = { v: raw, t: 1 }
      }
    }
  }
  ws.cellData = cellData
  if (colWidths?.length) {
    const columnData: MinimalWorksheetData['columnData'] = {}
    colWidths.forEach((w, i) => {
      if (w > 0) columnData[i] = { w }
    })
    ws.columnData = columnData
  }
  return ws
}

export function sheetsRowsToWorkbookData(
  sheets: Array<{ name?: string; rows?: string[][]; colWidths?: number[] }>,
  workbookName = 'Workbook',
): MinimalWorkbookData {
  const wb = emptyWorkbookData(workbookName)
  wb.sheetOrder = []
  wb.sheets = {}
  const list = sheets.length ? sheets : [{ name: 'Sheet1', rows: [['']] }]
  for (let i = 0; i < list.length; i++) {
    const s = list[i]
    const ws = rowsToWorksheetData(s.name || `Sheet${i + 1}`, s.rows?.length ? s.rows : [['']], s.colWidths)
    wb.sheetOrder.push(ws.id)
    wb.sheets[ws.id] = ws
  }
  return wb
}

/** Parse legacy `.danmo-sheet.json` body into IWorkbookData. */
export function migrateDanmoSheetJson(text: string): MinimalWorkbookData {
  const data = JSON.parse(text || '{}') as {
    sheets?: Array<{ name?: string; rows?: string[][]; colWidths?: number[] }>
  }
  return sheetsRowsToWorkbookData(data.sheets || [{ name: 'Sheet1', rows: [['']] }], 'Migrated')
}
