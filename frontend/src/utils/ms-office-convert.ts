import * as XLSX from 'xlsx'
import JSZip from 'jszip'
import { workbookFromAoASheets, textToDocumentData, pagesToSlideData, emptyDocumentData } from './univer-snapshots'
import type { MinimalWorkbookData } from './univer-workbook'

export async function xlsxArrayBufferToWorkbookData(buf: ArrayBuffer): Promise<MinimalWorkbookData> {
  const wb = XLSX.read(buf, { type: 'array' })
  const sheets = wb.SheetNames.map((name) => {
    const sheet = wb.Sheets[name]
    const rows = XLSX.utils.sheet_to_json<(string | number | boolean | null)[]>(sheet, {
      header: 1,
      defval: '',
      raw: false,
    }) as unknown[][]
    return { name, rows }
  })
  return workbookFromAoASheets(sheets.length ? sheets : [{ name: 'Sheet1', rows: [['']] }])
}

export async function docxArrayBufferToDocumentData(buf: ArrayBuffer): Promise<Record<string, unknown>> {
  const zip = await JSZip.loadAsync(buf)
  const xml = await zip.file('word/document.xml')?.async('string')
  if (!xml) return emptyDocumentData('Imported')
  const texts: string[] = []
  const paraRe = /<w:p[\s\S]*?<\/w:p>/g
  const paras = xml.match(paraRe) || []
  for (const p of paras) {
    const parts: string[] = []
    const tRe = /<w:t[^>]*>([^<]*)<\/w:t>/g
    let m: RegExpExecArray | null
    while ((m = tRe.exec(p))) parts.push(m[1])
    texts.push(parts.join(''))
  }
  return textToDocumentData(texts.join('\n'), 'Imported')
}

export async function pptxArrayBufferToSlideData(buf: ArrayBuffer): Promise<Record<string, unknown>> {
  const zip = await JSZip.loadAsync(buf)
  const slideFiles = Object.keys(zip.files)
    .filter((p) => /^ppt\/slides\/slide\d+\.xml$/i.test(p))
    .sort((a, b) => {
      const na = Number(a.match(/slide(\d+)/i)?.[1] || 0)
      const nb = Number(b.match(/slide(\d+)/i)?.[1] || 0)
      return na - nb
    })
  const pages: Array<{ title: string; body: string }> = []
  for (const path of slideFiles) {
    const xml = await zip.file(path)?.async('string')
    if (!xml) continue
    const texts: string[] = []
    const tRe = /<a:t[^>]*>([^<]*)<\/a:t>/g
    let m: RegExpExecArray | null
    while ((m = tRe.exec(xml))) texts.push(m[1])
    const title = texts[0] || `Slide ${pages.length + 1}`
    const body = texts.slice(1).join('\n')
    pages.push({ title, body })
  }
  return pagesToSlideData(pages.length ? pages : [{ title: 'Slide 1', body: '' }], 'Imported')
}
