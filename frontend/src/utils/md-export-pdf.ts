import html2pdf from 'html2pdf.js'
import { renderMarkdown } from '@/utils/markdown-render'

function basenamePdf(pathOrName: string): string {
  const base = pathOrName.replace(/\\/g, '/').split('/').pop() || 'document.md'
  return base.replace(/\.md$/i, '') + '.pdf'
}

/** Render Markdown to a temporary DOM node and download as PDF. */
export async function exportMarkdownPdf(markdown: string, pathOrName: string): Promise<void> {
  const filename = basenamePdf(pathOrName)
  const html = renderMarkdown(markdown || '')
  const host = document.createElement('div')
  host.className = 'md-export-pdf-host'
  host.setAttribute('aria-hidden', 'true')
  host.innerHTML = `<article class="md-export-pdf dq-prose">${html || '<p></p>'}</article>`
  Object.assign(host.style, {
    position: 'fixed',
    left: '-10000px',
    top: '0',
    width: '794px',
    padding: '0',
    margin: '0',
    background: '#fff',
    color: '#111',
    zIndex: '-1',
    pointerEvents: 'none',
  } as Partial<CSSStyleDeclaration>)
  document.body.appendChild(host)

  const article = host.querySelector('.md-export-pdf') as HTMLElement
  try {
    await html2pdf()
      .set({
        margin: [12, 12, 12, 12],
        filename,
        image: { type: 'jpeg', quality: 0.95 },
        html2canvas: { scale: 2, useCORS: true, backgroundColor: '#ffffff' },
        jsPDF: { unit: 'mm', format: 'a4', orientation: 'portrait' },
      })
      .from(article)
      .save()
  } finally {
    host.remove()
  }
}
