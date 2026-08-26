import type { MinimalWorkbookData } from './univer-workbook'
import { emptyWorkbookData, sheetsRowsToWorkbookData } from './univer-workbook'

/** Minimal IDocumentData for Univer Docs. */
export function emptyDocumentData(title = 'Document'): Record<string, unknown> {
  return {
    id: `doc_${Math.random().toString(36).slice(2, 10)}`,
    title,
    body: {
      dataStream: '\r\n',
      textRuns: [],
      paragraphs: [{ startIndex: 0 }],
      sectionBreaks: [{ startIndex: 1 }],
    },
    documentStyle: {
      pageSize: { width: 595.3, height: 841.9 },
      marginTop: 50,
      marginBottom: 50,
      marginLeft: 50,
      marginRight: 50,
    },
  }
}

export function textToDocumentData(text: string, title = 'Document'): Record<string, unknown> {
  const normalized = text.replace(/\r\n/g, '\n').replace(/\r/g, '\n')
  const paragraphs = normalized.split('\n')
  let dataStream = ''
  const paraMarks: Array<{ startIndex: number }> = []
  for (const line of paragraphs) {
    paraMarks.push({ startIndex: dataStream.length })
    dataStream += line + '\r'
  }
  dataStream += '\n'
  return {
    ...emptyDocumentData(title),
    body: {
      dataStream,
      textRuns: [],
      paragraphs: paraMarks,
      sectionBreaks: [{ startIndex: dataStream.length - 1 }],
    },
  }
}

function slidePage(id: string, title: string, bodyText: string, zIndex: number) {
  const titleId = `${id}_title`
  const bodyId = `${id}_body`
  return {
    id,
    pageType: 0,
    zIndex,
    title,
    description: '',
    pageBackgroundFill: { rgb: 'FFFFFF' },
    pageElements: {
      [titleId]: {
        id: titleId,
        zIndex: 1,
        left: 60,
        top: 80,
        width: 840,
        height: 80,
        title: 'title',
        description: '',
        type: 0,
        richText: { text: title, left: 60, top: 80, width: 840, height: 80 },
      },
      [bodyId]: {
        id: bodyId,
        zIndex: 2,
        left: 60,
        top: 200,
        width: 840,
        height: 360,
        title: 'body',
        description: '',
        type: 0,
        richText: { text: bodyText, left: 60, top: 200, width: 840, height: 360 },
      },
    },
  }
}

/** Minimal ISlideData with one title+body page. */
export function emptySlideData(deckTitle = 'Presentation'): Record<string, unknown> {
  const id = `slide_${Math.random().toString(36).slice(2, 10)}`
  const pageId = `${id}_p0`
  return {
    id,
    title: deckTitle,
    pageSize: { width: 960, height: 540 },
    body: {
      pageOrder: [pageId],
      pages: {
        [pageId]: slidePage(pageId, deckTitle, '', 0),
      },
    },
  }
}

export function pagesToSlideData(
  pages: Array<{ title: string; body: string }>,
  deckTitle = 'Presentation',
): Record<string, unknown> {
  const id = `slide_${Math.random().toString(36).slice(2, 10)}`
  const pageOrder: string[] = []
  const pageMap: Record<string, unknown> = {}
  const list = pages.length ? pages : [{ title: deckTitle, body: '' }]
  list.forEach((p, i) => {
    const pageId = `${id}_p${i}`
    pageOrder.push(pageId)
    pageMap[pageId] = slidePage(pageId, p.title || `Slide ${i + 1}`, p.body || '', i)
  })
  return {
    id,
    title: deckTitle,
    pageSize: { width: 960, height: 540 },
    body: { pageOrder, pages: pageMap },
  }
}

export function workbookFromAoASheets(
  sheets: Array<{ name: string; rows: unknown[][] }>,
): MinimalWorkbookData {
  return sheetsRowsToWorkbookData(
    sheets.map((s) => ({
      name: s.name,
      rows: s.rows.map((row) => row.map((c) => (c == null ? '' : String(c)))),
    })),
  )
}

export { emptyWorkbookData }
