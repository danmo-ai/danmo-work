import StarterKit from '@tiptap/starter-kit'
import { Placeholder } from '@tiptap/extensions'
import { Markdown } from '@tiptap/markdown'
import { TableKit } from '@tiptap/extension-table'
import TaskList from '@tiptap/extension-task-list'
import TaskItem from '@tiptap/extension-task-item'
import type { Extensions } from '@tiptap/core'

export interface OfficeDocExtensionOptions {
  placeholder?: string
}

/** Shared TipTap extensions for Document Stage + Knowledge (GFM MD SoT). */
export function createOfficeDocExtensions(opts: OfficeDocExtensionOptions = {}): Extensions {
  return [
    StarterKit.configure({
      link: { openOnClick: false },
    }),
    Placeholder.configure({
      placeholder: opts.placeholder || '开始编写…',
    }),
    TableKit.configure({
      table: { resizable: false },
    }),
    TaskList,
    TaskItem.configure({ nested: true }),
    Markdown,
  ]
}

export interface SlashCommandItem {
  id: string
  label: string
  description?: string
  /** Keywords for filter (lowercase). */
  keywords?: string[]
  run: (editor: import('@tiptap/core').Editor) => void
}

export function defaultSlashCommands(t: (key: string) => string): SlashCommandItem[] {
  return [
    {
      id: 'heading1',
      label: t('office.slashH1'),
      keywords: ['h1', 'title', '标题'],
      run: (ed) => ed.chain().focus().toggleHeading({ level: 1 }).run(),
    },
    {
      id: 'heading2',
      label: t('office.slashH2'),
      keywords: ['h2', '标题'],
      run: (ed) => ed.chain().focus().toggleHeading({ level: 2 }).run(),
    },
    {
      id: 'heading3',
      label: t('office.slashH3'),
      keywords: ['h3', '标题'],
      run: (ed) => ed.chain().focus().toggleHeading({ level: 3 }).run(),
    },
    {
      id: 'bullet',
      label: t('office.slashBullet'),
      keywords: ['ul', 'list', '列表'],
      run: (ed) => ed.chain().focus().toggleBulletList().run(),
    },
    {
      id: 'ordered',
      label: t('office.slashOrdered'),
      keywords: ['ol', 'list', '编号'],
      run: (ed) => ed.chain().focus().toggleOrderedList().run(),
    },
    {
      id: 'task',
      label: t('office.slashTask'),
      keywords: ['todo', 'checkbox', '任务'],
      run: (ed) => ed.chain().focus().toggleTaskList().run(),
    },
    {
      id: 'quote',
      label: t('office.slashQuote'),
      keywords: ['blockquote', '引用'],
      run: (ed) => ed.chain().focus().toggleBlockquote().run(),
    },
    {
      id: 'code',
      label: t('office.slashCode'),
      keywords: ['codeblock', '代码'],
      run: (ed) => ed.chain().focus().toggleCodeBlock().run(),
    },
    {
      id: 'table',
      label: t('office.slashTable'),
      keywords: ['table', '表格'],
      run: (ed) =>
        ed.chain().focus().insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run(),
    },
    {
      id: 'hr',
      label: t('office.slashHr'),
      keywords: ['divider', 'hr', '分割'],
      run: (ed) => ed.chain().focus().setHorizontalRule().run(),
    },
  ]
}

export interface TocHeading {
  id: string
  level: number
  text: string
  pos: number
}

/** Extract heading outline from the live editor document. */
export function extractTocHeadings(editor: import('@tiptap/core').Editor): TocHeading[] {
  const out: TocHeading[] = []
  editor.state.doc.descendants((node, pos) => {
    if (node.type.name !== 'heading') return
    const level = (node.attrs.level as number) || 1
    const text = node.textContent.trim()
    if (!text) return
    out.push({
      id: `h-${pos}`,
      level,
      text,
      pos,
    })
  })
  return out
}
