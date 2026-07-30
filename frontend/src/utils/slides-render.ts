/**
 * Marp-compatible subset → self-contained playable HTML for Office Slides Present.
 * Dialect: YAML frontmatter + `---` page breaks + GFM body + `<!-- notes: ... -->`.
 * HTML is a Stage derivative; Markdown remains the source of truth.
 */

import { renderMarkdown } from '@/utils/markdown-render'

export const SRC_HASH_COMMENT_RE = /<!--\s*danmo-slides-src-sha256:([a-f0-9]{64})\s*-->/i

export interface SlidePage {
  /** Markdown body without notes comments. */
  body: string
  notes: string
  /** Marp-compatible layout classes, e.g. lead, columns. */
  layoutClass: string
}

export interface ParsedSlidesMarkdown {
  /** Raw YAML frontmatter text (without enclosing ---), or empty. */
  frontmatterRaw: string
  title: string
  theme: string
  pages: SlidePage[]
}

/** Sibling playable HTML path for a slides Markdown path. */
export function siblingPlayableHtmlPath(mdPath: string): string {
  const normalized = mdPath.replace(/\\/g, '/')
  if (/\.markdown$/i.test(normalized)) {
    return normalized.replace(/\.markdown$/i, '.html')
  }
  if (/\.md$/i.test(normalized)) {
    return normalized.replace(/\.md$/i, '.html')
  }
  return `${normalized}.html`
}

/** Infer Markdown source path from a playable HTML path when possible. */
export function siblingSlidesMarkdownPath(htmlPath: string): string {
  const normalized = htmlPath.replace(/\\/g, '/')
  if (/\.html?$/i.test(normalized)) {
    return normalized.replace(/\.html?$/i, '.md')
  }
  return normalized
}

function stripNotes(body: string): { body: string; notes: string } {
  const notes: string[] = []
  const cleaned = body.replace(/<!--\s*notes:\s*([\s\S]*?)-->/gi, (_m, note: string) => {
    const t = String(note).trim()
    if (t) notes.push(t)
    return ''
  })
  return { body: cleaned.replace(/\n{3,}/g, '\n\n').trim(), notes: notes.join('\n') }
}

/** Extract Marp-style `<!-- _class: lead columns -->` / `<!-- class: … -->` from page body. */
function stripLayoutClass(body: string): { body: string; layoutClass: string } {
  let layoutClass = ''
  const cleaned = body.replace(/<!--\s*_?class\s*:\s*([^>]*?)-->/gi, (_m, cls: string) => {
    const t = String(cls).trim().replace(/\s+/g, ' ')
    if (t) layoutClass = layoutClass ? `${layoutClass} ${t}` : t
    return ''
  })
  return { body: cleaned.replace(/\n{3,}/g, '\n\n').trim(), layoutClass }
}

/**
 * Parse Marp-compatible slides Markdown.
 * First `---` … `---` block is frontmatter when it looks like YAML (contains `:`).
 */
export function parseSlidesMarkdown(md: string): ParsedSlidesMarkdown {
  const trimmed = md.replace(/^\uFEFF/, '').replace(/\r\n/g, '\n')
  let frontmatterRaw = ''
  let body = trimmed

  if (body.startsWith('---')) {
    const end = body.indexOf('\n---', 3)
    if (end !== -1) {
      const candidate = body.slice(4, end).trim()
      // Treat as frontmatter if it has a key: value line (Marp/YAML style).
      if (/^[a-zA-Z_][\w-]*\s*:/m.test(candidate)) {
        frontmatterRaw = candidate
        body = body.slice(end + 4).replace(/^\n+/, '')
      }
    }
  }

  const parts = body
    .split(/^\s*---\s*$/m)
    .map((p) => p.trim())
    .filter(Boolean)

  const pages = (parts.length ? parts : [body.trim() || '']).map((part) => {
    const withNotes = stripNotes(part)
    const withLayout = stripLayoutClass(withNotes.body)
    return { body: withLayout.body, notes: withNotes.notes, layoutClass: withLayout.layoutClass }
  })

  let title = ''
  let theme = 'default'
  for (const line of frontmatterRaw.split('\n')) {
    const m = line.match(/^([a-zA-Z_][\w-]*)\s*:\s*(.*)$/)
    if (!m) continue
    const key = m[1].toLowerCase()
    const val = m[2].trim().replace(/^["']|["']$/g, '')
    if (key === 'title') title = val
    if (key === 'theme') theme = val || 'default'
  }
  if (!title && pages[0]?.body) {
    const first = pages[0].body.split('\n').find((l) => l.trim())
    title = (first || '').replace(/^#+\s*/, '').trim()
  }

  return { frontmatterRaw, title, theme, pages }
}

/** Join page bodies back to Markdown (preserves notes as comments). Used by editor save. */
export function joinSlidePages(pages: string[], frontmatterRaw = ''): string {
  const body = pages.map((p) => p.trimEnd()).join('\n\n---\n\n') + '\n'
  if (!frontmatterRaw.trim()) return body
  return `---\n${frontmatterRaw.trim()}\n---\n\n${body}`
}

/** Split for the Stage editor (page strings may still include notes comments). */
export function splitSlidesForEditor(md: string): { frontmatterRaw: string; pages: string[] } {
  const trimmed = md.replace(/^\uFEFF/, '').replace(/\r\n/g, '\n')
  let frontmatterRaw = ''
  let body = trimmed

  if (body.startsWith('---')) {
    const end = body.indexOf('\n---', 3)
    if (end !== -1) {
      const candidate = body.slice(4, end).trim()
      if (/^[a-zA-Z_][\w-]*\s*:/m.test(candidate)) {
        frontmatterRaw = candidate
        body = body.slice(end + 4).replace(/^\n+/, '')
      }
    }
  }

  const parts = body
    .split(/^\s*---\s*$/m)
    .map((p) => p.trim())
    .filter(Boolean)

  return { frontmatterRaw, pages: parts.length ? parts : [body.trim() || ''] }
}

export async function hashSlidesSource(md: string): Promise<string> {
  const data = new TextEncoder().encode(md.replace(/\r\n/g, '\n'))
  const digest = await crypto.subtle.digest('SHA-256', data)
  return Array.from(new Uint8Array(digest))
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('')
}

export function extractSrcHashFromHtml(html: string): string | null {
  const m = html.match(SRC_HASH_COMMENT_RE)
  return m?.[1]?.toLowerCase() ?? null
}

export function isPlayableHtmlStale(html: string | null | undefined, mdHash: string): boolean {
  if (!html || !html.trim()) return true
  const existing = extractSrcHashFromHtml(html)
  if (!existing) return true
  return existing !== mdHash.toLowerCase()
}

function escapeAttr(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;')
}

/** Build self-contained playable HTML from slides Markdown + content hash. */
export function renderPlayableSlidesHtml(md: string, srcHash: string): string {
  const parsed = parseSlidesMarkdown(md)
  const title = parsed.title || 'Slides'
  const theme = (parsed.theme || 'default').toLowerCase()
  const slidesJson = JSON.stringify(
    parsed.pages.map((p) => ({
      html: renderMarkdown(p.body),
      notes: p.notes,
      layoutClass: p.layoutClass || '',
    })),
  )

  return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1"/>
<!-- danmo-slides-src-sha256:${srcHash.toLowerCase()} -->
<title>${escapeAttr(title)}</title>
<style>
  :root { color-scheme: light dark; --bg:#0f1115; --fg:#f3f4f6; --muted:#9ca3af; --accent:#60a5fa; --slide-bg:#161b22; --border:#30363d; }
  body[data-theme="light"] { color-scheme: light; --bg:#f6f4ef; --fg:#1c1917; --muted:#57534e; --accent:#2563eb; --slide-bg:#ffffff; --border:#d6d3d1; }
  body[data-theme="academic"] { color-scheme: light; --bg:#f8fafc; --fg:#0f172a; --muted:#475569; --accent:#0f766e; --slide-bg:#ffffff; --border:#cbd5e1; }
  * { box-sizing: border-box; }
  html, body { margin:0; height:100%; background:var(--bg); color:var(--fg);
    font-family: "Iowan Old Style", "Palatino Linotype", Palatino, Georgia, serif; }
  body[data-theme="academic"] { font-family: "Source Serif 4", "Times New Roman", Times, serif; }
  #deck { position:relative; width:100%; height:100%; overflow:hidden; }
  .slide { display:none; position:absolute; inset:0; padding:6vh 8vw; overflow:auto;
    background: radial-gradient(1200px 600px at 10% 0%, color-mix(in srgb, var(--accent) 18%, var(--slide-bg)) 0%, var(--slide-bg) 55%, var(--bg) 100%); }
  body[data-theme="light"] .slide, body[data-theme="academic"] .slide {
    background: linear-gradient(180deg, var(--slide-bg) 0%, color-mix(in srgb, var(--bg) 55%, var(--slide-bg)) 100%); }
  .slide.active { display:flex; flex-direction:column; justify-content:center; }
  .slide.lead { justify-content:center; text-align:center; align-items:center; }
  .slide.lead h1 { font-size: clamp(2.4rem, 6vw, 3.8rem); }
  .slide.lead > *:first-child { margin-top:0; }
  .slide.columns { display:none; }
  .slide.columns.active { display:grid; grid-template-columns:1fr 1fr; gap:2.5vw; align-items:start; justify-content:stretch; }
  .slide.columns > * { min-width:0; }
  .slide h1 { font-size: clamp(2rem, 5vw, 3.25rem); margin:0 0 0.4em; line-height:1.15; font-weight:700; }
  .slide h2 { font-size: clamp(1.5rem, 3.5vw, 2.25rem); margin:0 0 0.5em; line-height:1.2; }
  .slide h3 { font-size:1.35rem; margin:0 0 0.5em; }
  .slide p, .slide li { font-size: clamp(1.05rem, 2vw, 1.35rem); line-height:1.45;
    font-family: "Source Sans 3", "Helvetica Neue", Arial, sans-serif; }
  body[data-theme="academic"] .slide p, body[data-theme="academic"] .slide li {
    font-family: "Source Serif 4", Georgia, serif; }
  .slide ul, .slide ol { margin:0.2em 0 0; padding-left:1.2em; }
  .slide li { margin:0.35em 0; }
  .slide pre { background:color-mix(in srgb, var(--bg) 80%, #000); border:1px solid var(--border); border-radius:8px; padding:1em 1.2em;
    overflow:auto; font-size: clamp(0.9rem, 1.6vw, 1.1rem); line-height:1.4; }
  .slide code { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }
  .slide blockquote { margin:0; padding-left:1em; border-left:4px solid var(--accent); color:var(--muted); font-size:1.4rem; }
  .slide table { border-collapse:collapse; font-family: "Source Sans 3", Arial, sans-serif; }
  .slide th, .slide td { border:1px solid var(--border); padding:0.4em 0.7em; }
  .slide img { max-width:100%; max-height:55vh; object-fit:contain; }
  #bar { position:fixed; left:0; right:0; bottom:0; height:4px; background:var(--border); z-index:5; }
  #bar > i { display:block; height:100%; width:0; background:var(--accent); transition:width .2s; }
  #counter { position:fixed; right:16px; bottom:14px; z-index:6; font: 12px/1 ui-sans-serif, system-ui, sans-serif;
    color:var(--muted); letter-spacing:0.04em; }
  #presenter { display:none; position:fixed; inset:auto 16px 28px 16px; z-index:7; padding:12px 14px;
    background:rgba(0,0,0,.78); border:1px solid var(--border); border-radius:8px; max-height:30vh; overflow:auto;
    font: 13px/1.4 "Source Sans 3", Arial, sans-serif; color:#e5e7eb; white-space:pre-wrap; }
  #presenter.on { display:block; }
  @media print {
    #bar, #counter, #presenter { display:none !important; }
    .slide { display:flex !important; position:relative; page-break-after:always; height:100vh;
      background:#fff; color:#111; }
    .slide.columns { display:grid !important; }
  }
</style>
</head>
<body data-theme="${escapeAttr(theme)}">
<div id="deck"></div>
<div id="bar"><i id="progress"></i></div>
<div id="counter"></div>
<div id="presenter"></div>
<script>
const slides = ${slidesJson};
let idx = 0;
let presenter = false;
const deck = document.getElementById('deck');
const progress = document.getElementById('progress');
const counter = document.getElementById('counter');
const presenterEl = document.getElementById('presenter');

function mount() {
  deck.innerHTML = '';
  slides.forEach((s, i) => {
    const el = document.createElement('section');
    const layout = (s.layoutClass || '').trim();
    el.className = 'slide' + (layout ? ' ' + layout : '') + (i === 0 ? ' active' : '');
    el.dataset.index = String(i);
    el.innerHTML = s.html || '<p></p>';
    deck.appendChild(el);
  });
  go(parseHash() ?? 0, false);
}

function parseHash() {
  const m = location.hash.match(/^#(\\d+)$/);
  if (!m) return null;
  return Math.max(0, Math.min(slides.length - 1, parseInt(m[1], 10) - 1));
}

function go(n, pushHash) {
  if (!slides.length) return;
  idx = Math.max(0, Math.min(slides.length - 1, n));
  deck.querySelectorAll('.slide').forEach((el, i) => el.classList.toggle('active', i === idx));
  progress.style.width = ((idx + 1) / slides.length * 100) + '%';
  counter.textContent = (idx + 1) + ' / ' + slides.length;
  if (pushHash !== false) history.replaceState(null, '', '#' + (idx + 1));
  if (presenter) {
    const cur = slides[idx] || {};
    const next = slides[idx + 1];
    presenterEl.textContent = (cur.notes || '(no notes)') + (next ? '\\n\\n— next —\\n' + strip(next.html) : '');
  }
}

function strip(html) {
  const d = document.createElement('div');
  d.innerHTML = html || '';
  return (d.textContent || '').trim().slice(0, 200);
}

function togglePresenter() {
  presenter = !presenter;
  presenterEl.classList.toggle('on', presenter);
  go(idx, false);
}

window.addEventListener('keydown', (e) => {
  if (e.key === 'ArrowRight' || e.key === ' ' || e.key === 'PageDown') { e.preventDefault(); go(idx + 1); }
  else if (e.key === 'ArrowLeft' || e.key === 'PageUp') { e.preventDefault(); go(idx - 1); }
  else if (e.key === 'Home') { e.preventDefault(); go(0); }
  else if (e.key === 'End') { e.preventDefault(); go(slides.length - 1); }
  else if (e.key === 'p' || e.key === 'P') { e.preventDefault(); togglePresenter(); }
  else if (e.key === 'Escape' && presenter) { e.preventDefault(); togglePresenter(); }
});
window.addEventListener('hashchange', () => {
  const n = parseHash();
  if (n != null && n !== idx) go(n, false);
});
mount();
</script>
</body>
</html>
`
}
