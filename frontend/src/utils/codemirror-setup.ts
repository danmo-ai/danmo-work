import { Compartment, EditorState, type Extension } from '@codemirror/state'
import {
  EditorView,
  keymap,
  lineNumbers,
  highlightActiveLine,
  highlightActiveLineGutter,
  drawSelection,
  rectangularSelection,
  crosshairCursor,
} from '@codemirror/view'
import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands'
import {
  bracketMatching,
  defaultHighlightStyle,
  foldGutter,
  foldKeymap,
  indentOnInput,
  syntaxHighlighting,
  LanguageDescription,
} from '@codemirror/language'
import { highlightSelectionMatches, searchKeymap } from '@codemirror/search'
import { oneDark } from '@codemirror/theme-one-dark'
import { languages } from '@codemirror/language-data'

const langCompartment = () => new Compartment()
const readOnlyCompartment = () => new Compartment()
const themeCompartment = () => new Compartment()

/** Map languageFromPath() labels → filename hints for LanguageDescription.matchFilename. */
export function filenameHintForLanguage(path: string, language: string): string {
  const base = path.replace(/\\/g, '/').split('/').pop() || path
  if (base.includes('.')) return base
  switch (language) {
    case 'typescript':
      return 'file.ts'
    case 'javascript':
      return 'file.js'
    case 'python':
      return 'file.py'
    case 'go':
      return 'file.go'
    case 'rust':
      return 'file.rs'
    case 'shell':
      return 'file.sh'
    case 'yaml':
      return 'file.yaml'
    case 'json':
      return 'file.json'
    case 'markdown':
      return 'file.md'
    case 'html':
      return 'file.html'
    case 'css':
      return 'file.css'
    case 'sql':
      return 'file.sql'
    case 'dockerfile':
      return 'Dockerfile'
    case 'makefile':
      return 'Makefile'
    default:
      return base || 'file.txt'
  }
}

export async function loadLanguageExtension(path: string, language: string): Promise<Extension> {
  const hint = filenameHintForLanguage(path, language)
  const desc =
    LanguageDescription.matchFilename(languages, hint) ||
    LanguageDescription.matchLanguageName(languages, language, true)
  if (!desc) return []
  try {
    return await desc.load()
  } catch {
    return []
  }
}

export function createEditorTheme(dark: boolean): Extension {
  const chrome = EditorView.theme({
    '&': {
      height: '100%',
      fontSize: '13px',
    },
    '.cm-scroller': {
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace',
      lineHeight: '1.5',
      overflow: 'auto',
    },
    '.cm-content': {
      padding: '12px 0',
    },
  })
  if (dark) {
    return [oneDark, chrome]
  }
  return [
    syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
    chrome,
    EditorView.theme({
      '&': {
        backgroundColor: 'transparent',
        color: 'var(--dq-label-primary)',
      },
      '.cm-content': {
        caretColor: 'var(--dq-label-primary)',
      },
      '.cm-gutters': {
        backgroundColor: 'color-mix(in srgb, var(--dq-label-primary) 3%, transparent)',
        color: 'var(--dq-label-tertiary)',
        border: 'none',
        borderRight: '1px solid var(--dq-separator-light)',
      },
      '.cm-activeLineGutter': {
        backgroundColor: 'color-mix(in srgb, var(--dq-accent) 12%, transparent)',
      },
      '.cm-activeLine': {
        backgroundColor: 'color-mix(in srgb, var(--dq-label-primary) 4%, transparent)',
      },
      '&.cm-focused .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection': {
        backgroundColor: 'color-mix(in srgb, var(--dq-accent) 28%, transparent) !important',
      },
      '.cm-cursor, .cm-dropCursor': {
        borderLeftColor: 'var(--dq-accent)',
      },
    }),
  ]
}

export interface CodeMirrorHost {
  view: EditorView
  langComp: Compartment
  readOnlyComp: Compartment
  themeComp: Compartment
}

export function createCodeMirror(opts: {
  parent: HTMLElement
  doc: string
  readOnly: boolean
  dark: boolean
  languageExt: Extension
  onDocChanged: (doc: string) => void
  onSelectionChanged: (from: number, to: number) => void
}): CodeMirrorHost {
  const langComp = langCompartment()
  const readOnlyComp = readOnlyCompartment()
  const themeComp = themeCompartment()

  const view = new EditorView({
    parent: opts.parent,
    state: EditorState.create({
      doc: opts.doc,
      extensions: [
        lineNumbers(),
        highlightActiveLine(),
        highlightActiveLineGutter(),
        foldGutter(),
        drawSelection(),
        rectangularSelection(),
        crosshairCursor(),
        indentOnInput(),
        bracketMatching(),
        highlightSelectionMatches(),
        history(),
        keymap.of([...defaultKeymap, ...historyKeymap, ...foldKeymap, ...searchKeymap, indentWithTab]),
        langComp.of(opts.languageExt),
        readOnlyComp.of(EditorState.readOnly.of(opts.readOnly)),
        themeComp.of(createEditorTheme(opts.dark)),
        EditorView.updateListener.of((update) => {
          if (update.docChanged) {
            opts.onDocChanged(update.state.doc.toString())
          }
          if (update.selectionSet || update.docChanged) {
            const sel = update.state.selection.main
            opts.onSelectionChanged(sel.from, sel.to)
          }
        }),
        EditorView.domEventHandlers({
          blur: () => {
            const sel = view.state.selection.main
            opts.onSelectionChanged(sel.from, sel.to)
          },
        }),
      ],
    }),
  })

  return { view, langComp, readOnlyComp, themeComp }
}

export function setCodeMirrorDoc(host: CodeMirrorHost, doc: string) {
  const cur = host.view.state.doc.toString()
  if (cur === doc) return
  host.view.dispatch({
    changes: { from: 0, to: host.view.state.doc.length, insert: doc },
  })
}

export function setCodeMirrorReadOnly(host: CodeMirrorHost, readOnly: boolean) {
  host.view.dispatch({
    effects: host.readOnlyComp.reconfigure(EditorState.readOnly.of(readOnly)),
  })
}

export async function setCodeMirrorLanguage(host: CodeMirrorHost, path: string, language: string) {
  const ext = await loadLanguageExtension(path, language)
  host.view.dispatch({
    effects: host.langComp.reconfigure(ext),
  })
}

export function setCodeMirrorTheme(host: CodeMirrorHost, dark: boolean) {
  host.view.dispatch({
    effects: host.themeComp.reconfigure(createEditorTheme(dark)),
  })
}

export function getCodeMirrorSelection(host: CodeMirrorHost): { from: number; to: number; text: string } {
  const sel = host.view.state.selection.main
  return {
    from: sel.from,
    to: sel.to,
    text: host.view.state.sliceDoc(sel.from, sel.to),
  }
}
