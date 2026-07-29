<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useEditor, EditorContent } from '@tiptap/vue-3'
import type { Editor } from '@tiptap/core'
import { BubbleMenu } from '@tiptap/vue-3/menus'
import { useI18n } from 'vue-i18n'
import {
  createOfficeDocExtensions,
  defaultSlashCommands,
  extractTocHeadings,
  type SlashCommandItem,
  type TocHeading,
} from '@/utils/tiptap-extensions'
import { editorToMarkdown, selectionToMarkdown } from '@/utils/tiptap-markdown'

const props = withDefaults(
  defineProps<{
    /** Markdown content to display (controlled via setContent / watch). */
    content?: string
    editable?: boolean
    placeholder?: string
    /** Show sticky format toolbar when editable. */
    showToolbar?: boolean
    /** Show outline from headings (useful in view/read mode). */
    showToc?: boolean
  }>(),
  {
    content: '',
    editable: true,
    placeholder: '',
    showToolbar: true,
    showToc: true,
  },
)

const emit = defineEmits<{
  update: []
  selectionEmpty: [empty: boolean]
}>()

const { t } = useI18n()
const rootRef = ref<HTMLElement | null>(null)
const slashOpen = ref(false)
const slashQuery = ref('')
const slashIndex = ref(0)
const slashPos = ref<{ top: number; left: number } | null>(null)
const slashRange = ref<{ from: number; to: number } | null>(null)
const toc = ref<TocHeading[]>([])

const slashItems = computed(() => defaultSlashCommands((k) => t(k)))

const filteredSlash = computed(() => {
  const q = slashQuery.value.trim().toLowerCase()
  if (!q) return slashItems.value
  return slashItems.value.filter((item) => {
    const hay = [item.id, item.label, ...(item.keywords ?? [])].join('\n').toLowerCase()
    return hay.includes(q) || item.id.startsWith(q)
  })
})

const editor = useEditor({
  extensions: createOfficeDocExtensions({
    placeholder: props.placeholder || t('office.docPlaceholder'),
  }),
  content: props.content || '',
  contentType: 'markdown',
  editable: props.editable,
  onUpdate: ({ editor: ed }) => {
    const core = ed as unknown as Editor
    refreshToc(core)
    detectSlash(core)
    emit('update')
  },
  onSelectionUpdate: ({ editor: ed }) => {
    const core = ed as unknown as Editor
    emit('selectionEmpty', core.state.selection.empty)
    detectSlash(core)
  },
  editorProps: {
    handleKeyDown: (_view, event) => {
      if (!slashOpen.value) return false
      if (event.key === 'ArrowDown') {
        event.preventDefault()
        slashIndex.value = Math.min(slashIndex.value + 1, Math.max(0, filteredSlash.value.length - 1))
        return true
      }
      if (event.key === 'ArrowUp') {
        event.preventDefault()
        slashIndex.value = Math.max(slashIndex.value - 1, 0)
        return true
      }
      if (event.key === 'Enter' || event.key === 'Tab') {
        const item = filteredSlash.value[slashIndex.value]
        if (item) {
          event.preventDefault()
          runSlash(item)
          return true
        }
      }
      if (event.key === 'Escape') {
        closeSlash()
        return true
      }
      return false
    },
  },
})

function refreshToc(ed: Editor | null | undefined = editor.value as unknown as Editor | null) {
  if (!ed) {
    toc.value = []
    return
  }
  toc.value = extractTocHeadings(ed)
}

function detectSlash(ed: Editor) {
  if (!props.editable || !ed.isEditable) {
    closeSlash()
    return
  }
  const { $from, empty } = ed.state.selection
  if (!empty) {
    closeSlash()
    return
  }
  const textBefore = $from.parent.textBetween(0, $from.parentOffset, undefined, '\ufffc')
  const m = textBefore.match(/(^|[\s])\/([^\s]*)$/)
  if (!m) {
    closeSlash()
    return
  }
  const query = m[2] ?? ''
  const slashOffset = textBefore.length - query.length - 1
  const from = $from.start() + slashOffset
  const to = $from.pos
  slashQuery.value = query
  slashRange.value = { from, to }
  slashOpen.value = true
  slashIndex.value = 0
  const coords = ed.view.coordsAtPos(from)
  const rootBox = rootRef.value?.getBoundingClientRect()
  if (rootBox) {
    slashPos.value = {
      top: coords.bottom - rootBox.top + 6,
      left: Math.max(8, coords.left - rootBox.left),
    }
  }
}

function closeSlash() {
  slashOpen.value = false
  slashQuery.value = ''
  slashRange.value = null
  slashPos.value = null
}

function runSlash(item: SlashCommandItem) {
  const ed = editor.value
  if (!ed || !slashRange.value) return
  const { from, to } = slashRange.value
  ed.chain().focus().deleteRange({ from, to }).run()
  closeSlash()
  item.run(ed)
}

function setContent(md: string, opts?: { emitUpdate?: boolean }) {
  editor.value?.commands.setContent(md || '', {
    contentType: 'markdown',
    emitUpdate: opts?.emitUpdate ?? false,
  })
  refreshToc()
}

function getMarkdown(): string {
  if (!editor.value) return ''
  return editorToMarkdown(editor.value)
}

function getSelectionMarkdown(): string {
  if (!editor.value) return ''
  return selectionToMarkdown(editor.value)
}

function isSelectionEmpty(): boolean {
  return editor.value?.state.selection.empty ?? true
}

function scrollToHeading(h: TocHeading) {
  const ed = editor.value
  if (!ed) return
  ed.chain().focus().setTextSelection(h.pos + 1).run()
  const dom = ed.view.nodeDOM(h.pos)
  if (dom instanceof HTMLElement) {
    dom.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }
}

async function promptLink() {
  const ed = editor.value
  if (!ed) return
  const prev = (ed.getAttributes('link').href as string) || ''
  const href = window.prompt(t('office.linkPrompt'), prev)
  if (href === null) return
  if (!href.trim()) {
    ed.chain().focus().extendMarkRange('link').unsetLink().run()
    return
  }
  ed.chain().focus().extendMarkRange('link').setLink({ href: href.trim() }).run()
}

watch(
  () => props.editable,
  (editable) => {
    editor.value?.setEditable(editable)
    if (!editable) closeSlash()
  },
)

watch(
  () => props.content,
  (md) => {
    const ed = editor.value
    if (!ed) return
    if (normalizeCmp(md || '') === normalizeCmp(editorToMarkdown(ed))) return
    if (ed.isFocused && props.editable) return
    setContent(md || '', { emitUpdate: false })
  },
)

function normalizeCmp(md: string) {
  return md.replace(/\s+$/, '')
}

onBeforeUnmount(() => {
  editor.value?.destroy()
})

defineExpose({
  editor,
  setContent,
  getMarkdown,
  getSelectionMarkdown,
  isSelectionEmpty,
  refreshToc,
})
</script>

<template>
  <div
    ref="rootRef"
    class="md-rich"
    :class="{
      'is-readonly': !editable,
      'has-toc': showToc && toc.length > 0,
    }"
  >
    <aside v-if="showToc && toc.length > 0" class="md-rich__toc" aria-label="outline">
      <div class="md-rich__toc-title">{{ t('office.toc') }}</div>
      <button
        v-for="h in toc"
        :key="h.id"
        type="button"
        class="md-rich__toc-item"
        :class="`is-h${h.level}`"
        :title="h.text"
        @click="scrollToHeading(h)"
      >
        {{ h.text }}
      </button>
    </aside>

    <div class="md-rich__main">
      <div v-if="showToolbar && editable" class="md-rich__toolbar" role="toolbar">
        <button type="button" class="md-rich__btn" :title="t('office.fmtBold')" @click="editor?.chain().focus().toggleBold().run()">
          <strong>B</strong>
        </button>
        <button type="button" class="md-rich__btn" :title="t('office.fmtItalic')" @click="editor?.chain().focus().toggleItalic().run()">
          <em>I</em>
        </button>
        <button type="button" class="md-rich__btn" :title="t('office.fmtStrike')" @click="editor?.chain().focus().toggleStrike().run()">
          <s>S</s>
        </button>
        <button type="button" class="md-rich__btn" :title="t('office.fmtCode')" @click="editor?.chain().focus().toggleCode().run()">
          code
        </button>
        <span class="md-rich__sep" />
        <button type="button" class="md-rich__btn" :title="t('office.fmtH1')" @click="editor?.chain().focus().toggleHeading({ level: 1 }).run()">
          H1
        </button>
        <button type="button" class="md-rich__btn" :title="t('office.fmtH2')" @click="editor?.chain().focus().toggleHeading({ level: 2 }).run()">
          H2
        </button>
        <button type="button" class="md-rich__btn" :title="t('office.fmtH3')" @click="editor?.chain().focus().toggleHeading({ level: 3 }).run()">
          H3
        </button>
        <span class="md-rich__sep" />
        <button type="button" class="md-rich__btn" :title="t('office.slashBullet')" @click="editor?.chain().focus().toggleBulletList().run()">
          •
        </button>
        <button type="button" class="md-rich__btn" :title="t('office.slashOrdered')" @click="editor?.chain().focus().toggleOrderedList().run()">
          1.
        </button>
        <button type="button" class="md-rich__btn" :title="t('office.slashTask')" @click="editor?.chain().focus().toggleTaskList().run()">
          ☐
        </button>
        <button type="button" class="md-rich__btn" :title="t('office.slashQuote')" @click="editor?.chain().focus().toggleBlockquote().run()">
          “
        </button>
        <button type="button" class="md-rich__btn" :title="t('office.slashTable')" @click="editor?.chain().focus().insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run()">
          ▦
        </button>
        <button type="button" class="md-rich__btn" :title="t('office.fmtLink')" @click="promptLink">
          link
        </button>
      </div>

      <div class="md-rich__scroll">
        <EditorContent v-if="editor" :editor="editor" class="md-rich__editor dq-prose" />

        <BubbleMenu
          v-if="editor && editable"
          :editor="editor"
          :options="{ placement: 'top', offset: 8 }"
          class="md-rich__bubble"
        >
          <button type="button" class="md-rich__btn" @click="editor.chain().focus().toggleBold().run()">
            <strong>B</strong>
          </button>
          <button type="button" class="md-rich__btn" @click="editor.chain().focus().toggleItalic().run()">
            <em>I</em>
          </button>
          <button type="button" class="md-rich__btn" @click="editor.chain().focus().toggleCode().run()">
            code
          </button>
          <button type="button" class="md-rich__btn" @click="promptLink">link</button>
        </BubbleMenu>

        <div
          v-if="slashOpen && filteredSlash.length && slashPos"
          class="md-rich__slash"
          :style="{ top: `${slashPos.top}px`, left: `${slashPos.left}px` }"
        >
          <button
            v-for="(item, i) in filteredSlash"
            :key="item.id"
            type="button"
            class="md-rich__slash-item"
            :class="{ 'is-active': i === slashIndex }"
            @mousedown.prevent="runSlash(item)"
          >
            <span class="md-rich__slash-label">{{ item.label }}</span>
            <span class="md-rich__slash-id">/{{ item.id }}</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.md-rich {
  position: relative;
  display: flex;
  flex: 1;
  min-height: 0;
  min-width: 0;
  background: var(--dq-bg-base);
  color: var(--dq-label-primary);
}
.md-rich.has-toc {
  gap: 0;
}
.md-rich__toc {
  flex: 0 0 168px;
  border-right: 1px solid var(--dq-separator-light);
  padding: 16px 10px;
  overflow: auto;
  background: color-mix(in srgb, var(--dq-bg-elevated) 35%, transparent);
}
.md-rich__toc-title {
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--dq-label-tertiary);
  margin-bottom: 8px;
  padding: 0 6px;
}
.md-rich__toc-item {
  display: block;
  width: 100%;
  text-align: left;
  border: 0;
  background: transparent;
  color: var(--dq-label-secondary);
  font-size: 12px;
  line-height: 1.35;
  padding: 5px 6px;
  border-radius: 4px;
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.md-rich__toc-item:hover {
  background: color-mix(in srgb, var(--dq-label-primary) 8%, transparent);
  color: var(--dq-label-primary);
}
.md-rich__toc-item.is-h1 {
  font-weight: 600;
  color: var(--dq-label-primary);
}
.md-rich__toc-item.is-h2 {
  padding-left: 14px;
}
.md-rich__toc-item.is-h3,
.md-rich__toc-item.is-h4,
.md-rich__toc-item.is-h5,
.md-rich__toc-item.is-h6 {
  padding-left: 22px;
  font-size: 11px;
}
.md-rich__main {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
}
.md-rich__toolbar {
  flex-shrink: 0;
  display: flex;
  flex-wrap: wrap;
  gap: 2px;
  align-items: center;
  padding: 6px 10px;
  border-bottom: 1px solid var(--dq-separator-light);
  background: color-mix(in srgb, var(--dq-bg-elevated) 40%, transparent);
}
.md-rich__sep {
  width: 1px;
  height: 16px;
  margin: 0 4px;
  background: var(--dq-separator-light);
}
.md-rich__btn {
  height: 28px;
  min-width: 28px;
  padding: 0 7px;
  border: 0;
  border-radius: 5px;
  background: transparent;
  color: var(--dq-label-primary);
  font-size: 12px;
  cursor: pointer;
}
.md-rich__btn:hover {
  background: color-mix(in srgb, var(--dq-label-primary) 10%, transparent);
}
.md-rich__scroll {
  position: relative;
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 20px 24px 56px;
}
.md-rich.is-readonly .md-rich__scroll {
  padding-top: 28px;
  padding-bottom: 72px;
}
.md-rich__editor :deep(.tiptap) {
  outline: none;
  min-height: 240px;
  max-width: 720px;
  margin: 0 auto;
  font-size: 15px;
  line-height: 1.7;
  color: var(--dq-label-primary);
}
.md-rich.is-readonly .md-rich__editor :deep(.tiptap) {
  caret-color: transparent;
  font-size: 16px;
  line-height: 1.75;
  max-width: 680px;
}
.md-rich__editor :deep(.tiptap h1) {
  font-size: 1.85em;
  font-weight: 700;
  line-height: 1.25;
  margin: 1.2em 0 0.5em;
  letter-spacing: -0.02em;
}
.md-rich__editor :deep(.tiptap h2) {
  font-size: 1.4em;
  font-weight: 650;
  line-height: 1.3;
  margin: 1.15em 0 0.45em;
}
.md-rich__editor :deep(.tiptap h3) {
  font-size: 1.15em;
  font-weight: 600;
  margin: 1em 0 0.4em;
}
.md-rich__editor :deep(.tiptap p) {
  margin: 0.55em 0;
}
.md-rich__editor :deep(.tiptap blockquote) {
  margin: 0.8em 0;
  padding: 0.15em 0 0.15em 0.9em;
  border-left: 3px solid var(--dq-accent);
  color: var(--dq-label-secondary);
}
.md-rich__editor :deep(.tiptap pre) {
  margin: 0.8em 0;
  padding: 12px 14px;
  border-radius: 8px;
  background: color-mix(in srgb, var(--dq-fill-tertiary) 80%, transparent);
  overflow: auto;
  font-size: 13px;
}
.md-rich__editor :deep(.tiptap code) {
  font-family: var(--dq-font-mono, ui-monospace, SFMono-Regular, Menlo, monospace);
  font-size: 0.92em;
}
.md-rich__editor :deep(.tiptap p.is-editor-empty:first-child::before) {
  color: var(--dq-label-quaternary);
  content: attr(data-placeholder);
  float: left;
  height: 0;
  pointer-events: none;
}
.md-rich__editor :deep(.tiptap table) {
  border-collapse: collapse;
  width: 100%;
  margin: 0.9em 0;
  table-layout: fixed;
}
.md-rich__editor :deep(.tiptap th),
.md-rich__editor :deep(.tiptap td) {
  border: 1px solid var(--dq-border);
  padding: 6px 10px;
  vertical-align: top;
}
.md-rich__editor :deep(.tiptap th) {
  background: color-mix(in srgb, var(--dq-fill-tertiary) 70%, transparent);
  font-weight: 600;
}
.md-rich__editor :deep(.tiptap ul[data-type='taskList']) {
  list-style: none;
  padding-left: 0;
}
.md-rich__editor :deep(.tiptap ul[data-type='taskList'] li) {
  display: flex;
  align-items: flex-start;
  gap: 8px;
}
.md-rich__editor :deep(.tiptap ul[data-type='taskList'] li > label) {
  margin-top: 0.3em;
}
.md-rich__bubble {
  display: flex;
  gap: 2px;
  padding: 4px;
  border-radius: 8px;
  border: 1px solid var(--dq-border);
  background: var(--dq-bg-elevated);
  box-shadow: 0 6px 20px color-mix(in srgb, #000 18%, transparent);
}
.md-rich__slash {
  position: absolute;
  z-index: 20;
  min-width: 220px;
  max-height: 260px;
  overflow: auto;
  padding: 4px;
  border-radius: 8px;
  border: 1px solid var(--dq-border);
  background: var(--dq-bg-elevated);
  box-shadow: 0 8px 24px color-mix(in srgb, #000 16%, transparent);
}
.md-rich__slash-item {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  width: 100%;
  border: 0;
  background: transparent;
  color: var(--dq-label-primary);
  text-align: left;
  padding: 7px 8px;
  border-radius: 5px;
  cursor: pointer;
  font-size: 13px;
}
.md-rich__slash-item:hover,
.md-rich__slash-item.is-active {
  background: color-mix(in srgb, var(--dq-accent) 14%, transparent);
}
.md-rich__slash-id {
  color: var(--dq-label-tertiary);
  font-size: 11px;
  font-family: var(--dq-font-mono, ui-monospace, monospace);
}
@media (max-width: 720px) {
  .md-rich__toc {
    display: none;
  }
}
</style>
