import assert from 'node:assert/strict'
import { routeOfficeFile, isCodeFilePath, languageFromPath } from '../src/utils/office-route.ts'
import {
  selectionLineRange,
  serializeCodeSelectionAttachment,
  createCodeSelectionAttachment,
} from '../src/types/code-attachment.ts'

assert.equal(routeOfficeFile('src/main.go').kind, 'code')
assert.equal(routeOfficeFile('src/main.go').mode, 'view')
assert.equal(routeOfficeFile('app.ts').kind, 'code')
assert.equal(routeOfficeFile('notes.md').kind, 'doc')
assert.equal(routeOfficeFile('data.csv').kind, 'sheet')
assert.equal(routeOfficeFile('index.html').kind, 'preview')
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

console.log('office-route + code-attachment ok')
