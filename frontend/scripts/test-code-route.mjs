import assert from 'node:assert/strict'
import {
  routeProjectFile,
  isCodeFilePath,
  isWebFilePath,
  isMediaFilePath,
  languageFromPath,
  isUniverSheetPath,
} from '../src/utils/file-route.ts'

assert.equal(isCodeFilePath('src/main.go'), true)
assert.equal(isCodeFilePath('notes.md'), false)
assert.equal(isWebFilePath('index.html'), true)
assert.equal(isMediaFilePath('photo.png'), true)
assert.equal(isUniverSheetPath('report.usheet.json'), true)

assert.equal(routeProjectFile('src/main.go').kind, 'code')
assert.equal(routeProjectFile('src/main.go').mode, 'view')
assert.equal(routeProjectFile('src/main.go').engine, 'codemirror')
assert.equal(routeProjectFile('app.ts').kind, 'code')
assert.equal(routeProjectFile('notes.md').kind, 'doc')
assert.equal(routeProjectFile('notes.md').engine, 'md')

assert.equal(routeProjectFile('deck-slides.md').kind, 'doc')
assert.equal(routeProjectFile('slides/intro.md').kind, 'doc')
assert.equal(routeProjectFile('x.md', '---\ntype: slides\n---\n# Hi').kind, 'doc')
assert.equal(routeProjectFile('data.csv').kind, 'sheet')
assert.equal(routeProjectFile('data.csv').engine, 'csv')
assert.equal(routeProjectFile('report.usheet.json').kind, 'sheet')
assert.equal(routeProjectFile('report.usheet.json').engine, 'univer-sheet')
assert.equal(routeProjectFile('report.usheet.json').mode, 'edit')
assert.equal(routeProjectFile('deck.uslides.json').kind, 'slides')
assert.equal(routeProjectFile('deck.uslides.json').engine, 'univer-slides')
assert.equal(routeProjectFile('doc.udoc.json').kind, 'doc')
assert.equal(routeProjectFile('doc.udoc.json').engine, 'univer-doc')
assert.equal(routeProjectFile('a.xlsx').kind, 'sheet')
assert.equal(routeProjectFile('a.xlsx').engine, 'ms-office')
assert.equal(routeProjectFile('a.xlsx').mode, 'view')
assert.equal(routeProjectFile('a.docx').engine, 'ms-office')
assert.equal(routeProjectFile('a.pptx').kind, 'slides')
assert.equal(routeProjectFile('a.pptx').mode, 'view')
assert.equal(routeProjectFile('legacy.danmo-sheet.json').kind, 'sheet')
assert.equal(routeProjectFile('legacy.danmo-sheet.json').engine, 'univer-sheet')

assert.equal(routeProjectFile('index.html').kind, 'web')
assert.equal(routeProjectFile('index.html').engine, 'iframe')
assert.equal(routeProjectFile('deck-slides.html').kind, 'web')
assert.equal(routeProjectFile('photo.png').kind, 'media')
assert.equal(routeProjectFile('photo.png').engine, 'image')
assert.equal(routeProjectFile('logo.svg').kind, 'media')
assert.equal(routeProjectFile('assets/brand.SVG').kind, 'media')

assert.equal(languageFromPath('foo.ts'), 'typescript')
assert.equal(languageFromPath('bar.py'), 'python')

console.log('file-route ok')
