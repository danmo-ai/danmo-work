import assert from 'node:assert/strict'
import {
  routeOfficeFile,
  isCodeFilePath,
  languageFromPath,
  isUniverSheetPath,
  isLegacyDanmoSheetPath,
} from '../src/utils/office-route.ts'
import {
  selectionLineRange,
  serializeCodeSelectionAttachment,
  createCodeSelectionAttachment,
} from '../src/types/code-attachment.ts'
import { parseUniverFile, stringifyUniverFile, siblingUniverIrPath } from '../src/utils/univer-ir.ts'
import { migrateDanmoSheetJson, sheetsRowsToWorkbookData } from '../src/utils/univer-workbook.ts'

assert.equal(routeOfficeFile('src/main.go').kind, 'code')
assert.equal(routeOfficeFile('src/main.go').mode, 'view')
assert.equal(routeOfficeFile('src/main.go').engine, 'code')
assert.equal(routeOfficeFile('app.ts').kind, 'code')
assert.equal(routeOfficeFile('notes.md').kind, 'doc')
assert.equal(routeOfficeFile('notes.md').engine, 'md')
// Markdown is never slides — even with former Marp hints.
assert.equal(routeOfficeFile('deck-slides.md').kind, 'doc')
assert.equal(routeOfficeFile('slides/intro.md').kind, 'doc')
assert.equal(routeOfficeFile('x.md', '---\ntype: slides\n---\n# Hi').kind, 'doc')
assert.equal(routeOfficeFile('data.csv').kind, 'sheet')
assert.equal(routeOfficeFile('data.csv').engine, 'csv')
assert.equal(routeOfficeFile('report.usheet.json').kind, 'sheet')
assert.equal(routeOfficeFile('report.usheet.json').engine, 'univer-sheet')
assert.equal(routeOfficeFile('report.usheet.json').mode, 'edit')
assert.equal(routeOfficeFile('deck.uslides.json').kind, 'slides')
assert.equal(routeOfficeFile('deck.uslides.json').engine, 'univer-slides')
assert.equal(routeOfficeFile('doc.udoc.json').kind, 'doc')
assert.equal(routeOfficeFile('doc.udoc.json').engine, 'univer-doc')
assert.equal(routeOfficeFile('a.xlsx').kind, 'sheet')
assert.equal(routeOfficeFile('a.xlsx').engine, 'ms-office')
assert.equal(routeOfficeFile('a.xlsx').mode, 'view')
assert.equal(routeOfficeFile('a.docx').engine, 'ms-office')
assert.equal(routeOfficeFile('a.pptx').kind, 'slides')
assert.equal(routeOfficeFile('a.pptx').mode, 'view')
assert.equal(routeOfficeFile('legacy.danmo-sheet.json').kind, 'sheet')
assert.equal(routeOfficeFile('legacy.danmo-sheet.json').engine, 'univer-sheet')
assert.equal(isLegacyDanmoSheetPath('x.danmo-sheet.json'), true)
assert.equal(isUniverSheetPath('x.usheet.json'), true)
assert.equal(routeOfficeFile('index.html').kind, 'preview')
assert.equal(routeOfficeFile('deck-slides.html').kind, 'preview')
assert.equal(routeOfficeFile('photo.png').kind, 'preview')
assert.equal(routeOfficeFile('logo.svg').kind, 'preview')
assert.equal(routeOfficeFile('assets/brand.SVG').kind, 'preview')
assert.equal(isCodeFilePath('logo.svg'), false)
assert.ok(isCodeFilePath('Dockerfile'))
assert.equal(languageFromPath('foo/bar.ts'), 'typescript')

const text = 'a\nb\nc\n'
assert.deepEqual(selectionLineRange(text, 0, 1), { startLine: 1, endLine: 1 })
assert.deepEqual(selectionLineRange(text, 0, 3), { startLine: 1, endLine: 2 })
assert.deepEqual(selectionLineRange(text, 2, 5), { startLine: 2, endLine: 3 })

const att = createCodeSelectionAttachment({
  path: 'src/main.go',
  language: 'go',
  startLine: 10,
  endLine: 12,
  text: 'func main() {}',
  annotation: 'rename this',
})
const block = serializeCodeSelectionAttachment(att)
assert.match(block, /File: src\/main\.go/)
assert.match(block, /Lines: 10-12/)
assert.match(block, /Request: rename this/)
assert.match(block, /```go/)

const wb = sheetsRowsToWorkbookData([{ name: 'S1', rows: [['a', 'b'], ['1', '2']] }])
assert.equal(wb.sheetOrder.length, 1)
const sheet = wb.sheets[wb.sheetOrder[0]]
assert.equal(sheet.cellData[0][0].v, 'a')
assert.equal(sheet.cellData[1][1].v, 2)

const migrated = migrateDanmoSheetJson(
  JSON.stringify({ sheets: [{ name: 'Sheet1', rows: [['x']], colWidths: [120] }] }),
)
assert.equal(migrated.sheets[migrated.sheetOrder[0]].name, 'Sheet1')

const fileText = stringifyUniverFile('univer-sheet', wb)
const parsed = parseUniverFile(fileText, 'univer-sheet')
assert.equal(parsed.format, 'univer-sheet')
assert.equal(siblingUniverIrPath('reports/q1.xlsx', 'univer-sheet'), 'reports/q1.usheet.json')
assert.equal(siblingUniverIrPath('deck.pptx', 'univer-slides'), 'deck.uslides.json')

console.log('office-route + univer-ir ok')
