import TurndownService from 'turndown'
import { renderMarkdown } from '@/utils/markdown-render'

const turndown = new TurndownService({
  headingStyle: 'atx',
  codeBlockStyle: 'fenced',
  bulletListMarker: '-',
})

turndown.addRule('fencedCode', {
  filter: (node) =>
    node.nodeName === 'PRE' && node.firstChild != null && (node.firstChild as HTMLElement).nodeName === 'CODE',
  replacement: (_content, node) => {
    const code = (node as HTMLElement).querySelector('code')
    const cls = code?.getAttribute('class') || ''
    const lang = (cls.match(/language-([\w-]+)/) || [])[1] || ''
    const text = code?.textContent || ''
    return `\n\n\`\`\`${lang}\n${text.replace(/\n$/, '')}\n\`\`\`\n\n`
  },
})

export function markdownToEditorHTML(md: string): string {
  return renderMarkdown(md || '')
}

export function htmlToMarkdown(html: string): string {
  if (!html || html === '<p></p>') return ''
  return turndown.turndown(html).trim() + '\n'
}
