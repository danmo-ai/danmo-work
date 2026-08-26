import assert from 'node:assert/strict'
import { textToDocumentData, pagesToSlideData, emptySlideData } from '../src/utils/univer-snapshots.ts'
import { xlsxArrayBufferToWorkbookData } from '../src/utils/ms-office-convert.ts'
import * as XLSX from 'xlsx'

const doc = textToDocumentData('Hello\nWorld', 'T')
assert.ok(String(doc.body.dataStream).includes('Hello'))

const slides = pagesToSlideData([{ title: 'A', body: 'b1' }, { title: 'B', body: 'b2' }], 'Deck')
assert.equal(slides.body.pageOrder.length, 2)
assert.ok(emptySlideData('X').title)

const wb = XLSX.utils.book_new()
const ws = XLSX.utils.aoa_to_sheet([
  ['Name', 'Score'],
  ['Ada', 10],
])
XLSX.utils.book_append_sheet(wb, ws, 'Grades')
const buf = XLSX.write(wb, { type: 'array', bookType: 'xlsx' })
const snapshot = await xlsxArrayBufferToWorkbookData(buf)
assert.equal(snapshot.sheetOrder.length, 1)
const sheet = snapshot.sheets[snapshot.sheetOrder[0]]
assert.equal(sheet.name, 'Grades')
assert.equal(sheet.cellData[0][0].v, 'Name')
assert.equal(sheet.cellData[1][1].v, 10)

console.log('ms-office-convert + snapshots ok')
