import html2pdf from 'html2pdf.js'
import { renderMarkdown } from '@/utils/markdown-render'
import { saveBlobAs, type SaveBlobResult } from '@/utils/desktop'

/** Print-only styles: hex/rgb only (no color-mix / color() / CSS vars). */
const PRINT_CSS = `
.md-export-pdf {
  box-sizing: border-box;
  width: 100%;
  padding: 8px 4px;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial,
    "Noto Sans", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif;
  font-size: 14px;
  line-height: 1.65;
  color: #111111;
  background: #ffffff;
  word-break: break-word;
}
.md-export-pdf *,
.md-export-pdf *::before,
.md-export-pdf *::after {
  box-sizing: border-box;
}
.md-export-pdf :first-child { margin-top: 0; }
.md-export-pdf :last-child { margin-bottom: 0; }
.md-export-pdf h1,
.md-export-pdf h2,
.md-export-pdf h3,
.md-export-pdf h4,
.md-export-pdf h5,
.md-export-pdf h6 {
  margin: 1.15em 0 0.45em;
  font-weight: 650;
  line-height: 1.3;
  color: #111111;
}
.md-export-pdf h1 { font-size: 1.55em; }
.md-export-pdf h2 { font-size: 1.35em; }
.md-export-pdf h3 { font-size: 1.2em; }
.md-export-pdf h4,
.md-export-pdf h5,
.md-export-pdf h6 { font-size: 1em; font-weight: 600; }
.md-export-pdf p { margin: 0.65em 0; }
.md-export-pdf ul,
.md-export-pdf ol { margin: 0.5em 0; padding-left: 1.4em; }
.md-export-pdf li + li { margin-top: 0.25em; }
.md-export-pdf blockquote {
  margin: 0.65em 0;
  padding: 0.35em 0 0.35em 0.85em;
  border-left: 3px solid #2563eb;
  color: #4b5563;
}
.md-export-pdf a { color: #2563eb; text-decoration: none; }
.md-export-pdf strong { font-weight: 600; color: #111111; }
.md-export-pdf em { font-style: italic; color: #374151; }
.md-export-pdf table {
  width: 100%;
  border-collapse: collapse;
  margin: 0.75em 0;
  font-size: 13px;
  border: 1px solid #d1d5db;
}
.md-export-pdf th,
.md-export-pdf td {
  border: 1px solid #e5e7eb;
  padding: 0.4em 0.65em;
  text-align: left;
  line-height: 1.5;
}
.md-export-pdf th {
  background: #f3f4f6;
  font-weight: 600;
  color: #111111;
}
.md-export-pdf hr {
  border: none;
  border-top: 1px solid #d1d5db;
  margin: 1.2em 0;
}
.md-export-pdf img { max-width: 100%; margin: 0.5em 0; }
.md-export-pdf code {
  padding: 0.1em 0.35em;
  border-radius: 4px;
  background: #f3f4f6;
  font-family: ui-monospace, "SF Mono", Menlo, Consolas, monospace;
  font-size: 0.9em;
  color: #111111;
}
.md-export-pdf pre {
  margin: 0.75em 0;
  padding: 0.75em 0.9em;
  border-radius: 6px;
  border: 1px solid #e5e7eb;
  background: #f8fafc;
  overflow-x: auto;
  font-family: ui-monospace, "SF Mono", Menlo, Consolas, monospace;
  font-size: 12.5px;
  line-height: 1.55;
  color: #111111;
}
.md-export-pdf pre code {
  padding: 0;
  background: transparent;
  border: none;
  font-size: inherit;
  line-height: inherit;
  color: inherit;
}
`

function basenamePdf(pathOrName: string): string {
  const base = pathOrName.replace(/\\/g, '/').split('/').pop() || 'document.md'
  return base.replace(/\.md$/i, '') + '.pdf'
}

/** Prefer same folder as the source .md when path looks absolute. */
function defaultPdfPath(pathOrName: string): string {
  const filename = basenamePdf(pathOrName)
  const norm = pathOrName.replace(/\\/g, '/')
  const slash = norm.lastIndexOf('/')
  if (slash <= 0) return filename
  // Absolute (Unix /… or Windows C:/…)
  if (norm.startsWith('/') || /^[A-Za-z]:\//.test(norm)) {
    return `${norm.slice(0, slash + 1)}${filename}`
  }
  return filename
}

async function renderMarkdownPdfBlob(markdown: string, filename: string): Promise<Blob> {
  const html = renderMarkdown(markdown || '')
  const host = document.createElement('div')
  host.className = 'md-export-pdf-host'
  host.setAttribute('aria-hidden', 'true')
  host.innerHTML = `<style>${PRINT_CSS}</style><article class="md-export-pdf">${html || '<p></p>'}</article>`
  Object.assign(host.style, {
    position: 'fixed',
    left: '-10000px',
    top: '0',
    width: '794px',
    padding: '0',
    margin: '0',
    background: '#ffffff',
    color: '#111111',
    zIndex: '-1',
    pointerEvents: 'none',
  } as Partial<CSSStyleDeclaration>)
  document.body.appendChild(host)

  const article = host.querySelector('.md-export-pdf') as HTMLElement
  try {
    return (await html2pdf()
      .set({
        margin: [12, 12, 12, 12],
        filename,
        image: { type: 'jpeg', quality: 0.95 },
        html2canvas: {
          scale: 2,
          useCORS: true,
          backgroundColor: '#ffffff',
          // Clone may still inherit app stylesheets; keep print colors only.
          onclone: (doc: Document) => {
            const style = doc.createElement('style')
            style.textContent = PRINT_CSS
            doc.head.appendChild(style)
          },
        },
        jsPDF: { unit: 'mm', format: 'a4', orientation: 'portrait' },
      })
      .from(article)
      .outputPdf('blob')) as Blob
  } finally {
    host.remove()
  }
}

/** Render Markdown to PDF and prompt for a save location when possible. */
export async function exportMarkdownPdf(
  markdown: string,
  pathOrName: string,
): Promise<SaveBlobResult> {
  const filename = basenamePdf(pathOrName)
  // Dialog first (user gesture), then render — cancel skips the heavy work.
  return saveBlobAs(() => renderMarkdownPdfBlob(markdown, filename), filename, {
    filters: [{ name: 'PDF', extensions: ['pdf'] }],
    defaultPath: defaultPdfPath(pathOrName),
  })
}
