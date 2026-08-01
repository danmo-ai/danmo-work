/**
 * Marp-compatible subset → self-contained playable HTML for Office Slides Present.
 * Dialect: YAML frontmatter + `---` page breaks + GFM body + `<!-- notes: ... -->`.
 * Present shell borrows Reveal.js ideas (fragments, fade, overview) without depending on it.
 * HTML is a Stage derivative; Markdown remains the source of truth.
 */

import { renderMarkdown } from '@/utils/markdown-render'

export const SRC_HASH_COMMENT_RE = /<!--\s*danmo-slides-src-sha256:([a-f0-9]{64})\s*-->/i

export interface SlidePage {
  /** Markdown body without notes / layout / page-option comments. */
  body: string
  notes: string
  /** Marp-compatible layout classes, e.g. lead, columns. */
  layoutClass: string
  /** Per-page auto-fragment lists (overrides deck default when set). */
  fragments?: boolean
}

export interface ParsedSlidesMarkdown {
  /** Raw YAML frontmatter text (without enclosing ---), or empty. */
  frontmatterRaw: string
  title: string
  theme: string
  /** Deck-wide: auto-mark list items as fragments (Reveal-inspired). */
  fragments: boolean
  /** Present transition: fade | none */
  transition: 'fade' | 'none'
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

/** Page option `<!-- fragments: true|false -->` (Reveal-inspired stepped reveals). */
function stripPageFragments(body: string): { body: string; fragments?: boolean } {
  let fragments: boolean | undefined
  const cleaned = body.replace(/<!--\s*fragments\s*:\s*(true|false|on|off|1|0)\s*-->/gi, (_m, v: string) => {
    const x = String(v).toLowerCase()
    fragments = x === 'true' || x === 'on' || x === '1'
    return ''
  })
  return { body: cleaned.replace(/\n{3,}/g, '\n\n').trim(), fragments }
}

function parseBool(val: string): boolean {
  const v = val.trim().toLowerCase()
  return v === 'true' || v === 'yes' || v === 'on' || v === '1'
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
    const withFrag = stripPageFragments(withLayout.body)
    const page: SlidePage = {
      body: withFrag.body,
      notes: withNotes.notes,
      layoutClass: withLayout.layoutClass,
    }
    if (withFrag.fragments != null) page.fragments = withFrag.fragments
    return page
  })

  let title = ''
  let theme = 'default'
  let fragments = false
  let transition: 'fade' | 'none' = 'fade'
  for (const line of frontmatterRaw.split('\n')) {
    const m = line.match(/^([a-zA-Z_][\w-]*)\s*:\s*(.*)$/)
    if (!m) continue
    const key = m[1].toLowerCase()
    const val = m[2].trim().replace(/^["']|["']$/g, '')
    if (key === 'title') title = val
    if (key === 'theme') theme = val || 'default'
    if (key === 'fragments') fragments = parseBool(val)
    if (key === 'transition') {
      const t = val.toLowerCase()
      transition = t === 'none' || t === 'false' || t === 'off' ? 'none' : 'fade'
    }
  }
  if (!title && pages[0]?.body) {
    const first = pages[0].body.split('\n').find((l) => l.trim())
    title = (first || '').replace(/^#+\s*/, '').trim()
  }

  return { frontmatterRaw, title, theme, fragments, transition, pages }
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

/**
 * Reveal-inspired: mark list items as `.fragment` for stepped reveal.
 * Skips items that already have a fragment class. Exported for unit tests.
 */
export function applyAutoFragments(html: string, enabled: boolean): string {
  if (!enabled || !html) return html
  return html.replace(/<li(\s[^>]*)?>/gi, (full, attrs: string | undefined) => {
    const a = attrs || ''
    if (/\bfragment\b/i.test(a)) return full
    if (/class\s*=\s*(["'])/i.test(a)) {
      return full.replace(/class\s*=\s*(["'])/i, 'class=$1fragment ')
    }
    return `<li class="fragment"${a}>`
  })
}

/** Build self-contained playable HTML from slides Markdown + content hash. */
export function renderPlayableSlidesHtml(md: string, srcHash: string): string {
  const parsed = parseSlidesMarkdown(md)
  const title = parsed.title || 'Slides'
  const theme = (parsed.theme || 'default').toLowerCase()
  const transition = parsed.transition
  const slidesJson = JSON.stringify(
    parsed.pages.map((p) => {
      const autoFrag = p.fragments != null ? p.fragments : parsed.fragments
      return {
        html: applyAutoFragments(renderMarkdown(p.body), autoFrag),
        notes: p.notes,
        layoutClass: p.layoutClass || '',
      }
    }),
  )

  return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1"/>
<!-- danmo-slides-src-sha256:${srcHash.toLowerCase()} -->
<title>${escapeAttr(title)}</title>
<style>
  :root {
    color-scheme: light dark;
    --bg:#0f1115; --fg:#f3f4f6; --muted:#9ca3af; --accent:#60a5fa;
    --slide-bg:#161b22; --border:#30363d; --code-bg:#0b0f14;
    --frag-hidden-opacity: 0; --transition-ms: 280ms;
  }
  body[data-theme="light"] {
    color-scheme: light;
    --bg:#f4f1ea; --fg:#1c1917; --muted:#57534e; --accent:#1d4ed8;
    --slide-bg:#fffcf7; --border:#d6d3d1; --code-bg:#f5f5f4;
  }
  body[data-theme="academic"] {
    color-scheme: light;
    --bg:#f1f5f9; --fg:#0f172a; --muted:#475569; --accent:#0f766e;
    --slide-bg:#ffffff; --border:#cbd5e1; --code-bg:#f8fafc;
  }
  body[data-theme="moon"] {
    color-scheme: dark;
    --bg:#1a1b26; --fg:#c0caf5; --muted:#565f89; --accent:#7aa2f7;
    --slide-bg:#24283b; --border:#3b4261; --code-bg:#1f2335;
  }
  * { box-sizing: border-box; }
  html, body { margin:0; height:100%; background:var(--bg); color:var(--fg);
    font-family: "Iowan Old Style", "Palatino Linotype", Palatino, Georgia, serif; }
  body[data-theme="academic"] { font-family: "Source Serif 4", "Times New Roman", Times, serif; }
  body[data-theme="moon"] { font-family: "Source Sans 3", "Helvetica Neue", Arial, sans-serif; }
  #deck { position:relative; width:100%; height:100%; overflow:hidden; }
  .slide {
    display:flex; flex-direction:column; justify-content:center;
    position:absolute; inset:0; padding:6vh 8vw; overflow:auto;
    background: radial-gradient(1200px 600px at 10% 0%, color-mix(in srgb, var(--accent) 18%, var(--slide-bg)) 0%, var(--slide-bg) 55%, var(--bg) 100%);
    opacity:0; pointer-events:none; z-index:0;
  }
  body[data-transition="fade"] .slide {
    transition: opacity var(--transition-ms) ease, transform var(--transition-ms) ease;
    transform: translateY(8px);
  }
  body[data-theme="light"] .slide, body[data-theme="academic"] .slide {
    background: linear-gradient(180deg, var(--slide-bg) 0%, color-mix(in srgb, var(--bg) 55%, var(--slide-bg)) 100%);
  }
  .slide.active { opacity:1; pointer-events:auto; z-index:1; transform:none; }
  .slide.lead { text-align:center; align-items:center; }
  .slide { font-size: var(--dq-font-size-prose); }
  .slide.lead h1 { font-size: 2.4em; letter-spacing:-0.02em; }
  .slide.lead > *:first-child { margin-top:0; }
  .slide.columns.active { display:grid; grid-template-columns:1fr 1fr; gap:2.5vw; align-items:start; justify-content:stretch; }
  .slide.columns > * { min-width:0; }
  .slide h1 { font-size: 2em; margin:0 0 0.4em; line-height:1.15; font-weight:700; }
  .slide h2 { font-size: 1.5em; margin:0 0 0.5em; line-height:1.2; }
  .slide h3 { font-size: var(--dq-font-size-title); margin:0 0 0.5em; }
  .slide p, .slide li { font-size: 1em; line-height:1.45;
    font-family: "Source Sans 3", "Helvetica Neue", Arial, sans-serif; }
  body[data-theme="academic"] .slide p, body[data-theme="academic"] .slide li {
    font-family: "Source Serif 4", Georgia, serif; }
  .slide ul, .slide ol { margin:0.2em 0 0; padding-left:1.2em; }
  .slide li { margin:0.35em 0; }
  .slide pre { background:var(--code-bg); border:1px solid var(--border); border-radius:10px; padding:1em 1.2em;
    overflow:auto; font-size: 0.9em; line-height:1.45; box-shadow: 0 1px 0 color-mix(in srgb, #000 12%, transparent); }
  .slide code { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }
  .slide :not(pre) > code { background: color-mix(in srgb, var(--accent) 12%, transparent); padding:0.1em 0.35em; border-radius:4px; font-size: var(--dq-font-size-caption); }
  .slide blockquote { margin:0; padding-left:1em; border-left:4px solid var(--accent); color:var(--muted); font-size: var(--dq-font-size-title); }
  .slide table { border-collapse:collapse; font-family: "Source Sans 3", Arial, sans-serif; width:auto; max-width:100%; }
  .slide th, .slide td { border:1px solid var(--border); padding:0.45em 0.8em; }
  .slide th { background: color-mix(in srgb, var(--accent) 10%, transparent); text-align:left; }
  .slide img { max-width:100%; max-height:55vh; object-fit:contain; border-radius:6px; }

  /* Reveal-inspired fragments */
  .fragment { opacity: var(--frag-hidden-opacity); transition: opacity 0.22s ease, transform 0.22s ease; transform: translateY(0.35em); }
  .fragment.visible { opacity: 1; transform: none; }
  .fragment.current-fragment { outline: none; }

  #bar { position:fixed; left:0; right:0; bottom:0; height:4px; background:var(--border); z-index:5; }
  #bar > i { display:block; height:100%; width:0; background:var(--accent); transition:width .2s; }
  #counter { position:fixed; right:16px; bottom:14px; z-index:6;
    font-family: ui-sans-serif, system-ui, sans-serif; font-size: var(--dq-font-size-caption); line-height: 1;
    color:var(--muted); letter-spacing:0.04em; }
  #hint { position:fixed; left:16px; bottom:14px; z-index:6;
    font-family: ui-sans-serif, system-ui, sans-serif; font-size: var(--dq-font-size-caption); line-height: 1.3;
    color:var(--muted); opacity:0.75; }
  #presenter { display:none; position:fixed; inset:auto 16px 28px 16px; z-index:7; padding:12px 14px;
    background:rgba(0,0,0,.78); border:1px solid var(--border); border-radius:8px; max-height:32vh; overflow:auto;
    font-family: "Source Sans 3", Arial, sans-serif; font-size: var(--dq-font-size-body); line-height: 1.4;
    color:#e5e7eb; white-space:pre-wrap; }
  #presenter.on { display:block; }
  #presenter .meta { color:#9ca3af; font-size: var(--dq-font-size-caption); margin-bottom:6px; }

  /* Overview (Reveal-inspired) */
  body.is-overview #deck { display:grid; grid-template-columns:repeat(auto-fill, minmax(220px, 1fr)); gap:14px;
    padding:24px; overflow:auto; align-content:start; }
  body.is-overview .slide {
    position:relative; inset:auto; display:flex !important; opacity:1 !important; pointer-events:auto;
    height:140px; padding:14px 16px; transform:none !important; border:1px solid var(--border); border-radius:10px;
    cursor:pointer; overflow:hidden; font-size:40% !important; z-index:1;
  }
  body.is-overview .slide.active { outline: 2px solid var(--accent); }
  body.is-overview .fragment { opacity:1 !important; transform:none !important; }
  body.is-overview #bar, body.is-overview #hint, body.is-overview #presenter { display:none !important; }
  body.is-overview #counter { bottom:auto; top:14px; }

  @media print {
    #bar, #counter, #hint, #presenter { display:none !important; }
    body.is-overview #deck { display:block; padding:0; }
    .slide { display:flex !important; position:relative; page-break-after:always; height:100vh;
      background:#fff !important; color:#111 !important; opacity:1 !important; transform:none !important; pointer-events:auto; }
    .slide.columns { display:grid !important; }
    .fragment { opacity:1 !important; transform:none !important; }
  }
</style>
</head>
<body data-theme="${escapeAttr(theme)}" data-transition="${escapeAttr(transition)}">
<div id="deck"></div>
<div id="bar"><i id="progress"></i></div>
<div id="counter"></div>
<div id="hint">Space · fragments → next · P notes · O overview</div>
<div id="presenter"></div>
<script>
const slides = ${slidesJson};
let idx = 0;
let presenter = false;
let overview = false;
let startedAt = Date.now();
const deck = document.getElementById('deck');
const progress = document.getElementById('progress');
const counter = document.getElementById('counter');
const presenterEl = document.getElementById('presenter');

function mount() {
  deck.innerHTML = '';
  slides.forEach((s, i) => {
    const el = document.createElement('section');
    const layout = (s.layoutClass || '').trim();
    el.className = 'slide' + (layout ? ' ' + layout : '');
    el.dataset.index = String(i);
    el.innerHTML = s.html || '<p></p>';
    el.addEventListener('click', () => {
      if (overview) {
        setOverview(false);
        go(i);
      }
    });
    deck.appendChild(el);
  });
  go(parseHash() ?? 0, false);
}

function parseHash() {
  const m = location.hash.match(/^#(\\d+)$/);
  if (!m) return null;
  return Math.max(0, Math.min(slides.length - 1, parseInt(m[1], 10) - 1));
}

function slideEls() {
  return Array.from(deck.querySelectorAll('.slide'));
}

function currentSlide() {
  return slideEls()[idx];
}

function fragmentsOf(slide) {
  if (!slide) return [];
  return Array.from(slide.querySelectorAll('.fragment'));
}

function resetFragments(slide) {
  fragmentsOf(slide).forEach((f) => {
    f.classList.remove('visible', 'current-fragment');
  });
}

function nextFragment() {
  const slide = currentSlide();
  const frags = fragmentsOf(slide);
  const next = frags.find((f) => !f.classList.contains('visible'));
  if (!next) return false;
  frags.forEach((f) => f.classList.remove('current-fragment'));
  next.classList.add('visible', 'current-fragment');
  return true;
}

function prevFragment() {
  const slide = currentSlide();
  const frags = fragmentsOf(slide);
  const visible = frags.filter((f) => f.classList.contains('visible'));
  if (!visible.length) return false;
  const last = visible[visible.length - 1];
  last.classList.remove('visible', 'current-fragment');
  const still = frags.filter((f) => f.classList.contains('visible'));
  if (still.length) still[still.length - 1].classList.add('current-fragment');
  return true;
}

function elapsed() {
  const sec = Math.floor((Date.now() - startedAt) / 1000);
  const m = String(Math.floor(sec / 60)).padStart(2, '0');
  const s = String(sec % 60).padStart(2, '0');
  return m + ':' + s;
}

function updateChrome() {
  progress.style.width = slides.length ? ((idx + 1) / slides.length * 100) + '%' : '0%';
  counter.textContent = (idx + 1) + ' / ' + slides.length + (overview ? ' · overview' : '');
  if (presenter) {
    const cur = slides[idx] || {};
    const next = slides[idx + 1];
    presenterEl.innerHTML =
      '<div class="meta">Speaker · ' + elapsed() + ' · slide ' + (idx + 1) + '/' + slides.length + '</div>' +
      escapeHtml(cur.notes || '(no notes)') +
      (next ? '\\n\\n— next —\\n' + escapeHtml(strip(next.html)) : '');
  }
}

function escapeHtml(s) {
  return String(s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

function go(n, pushHash) {
  if (!slides.length) return;
  const prev = idx;
  idx = Math.max(0, Math.min(slides.length - 1, n));
  slideEls().forEach((el, i) => {
    const on = i === idx;
    el.classList.toggle('active', on);
    if (on && i !== prev) resetFragments(el);
  });
  if (pushHash !== false) history.replaceState(null, '', '#' + (idx + 1));
  updateChrome();
}

function strip(html) {
  const d = document.createElement('div');
  d.innerHTML = html || '';
  return (d.textContent || '').trim().slice(0, 200);
}

function togglePresenter() {
  presenter = !presenter;
  presenterEl.classList.toggle('on', presenter);
  if (presenter) startedAt = Date.now();
  updateChrome();
}

function setOverview(on) {
  overview = !!on;
  document.body.classList.toggle('is-overview', overview);
  updateChrome();
}

function toggleOverview() {
  setOverview(!overview);
}

function forward() {
  if (overview) { setOverview(false); return; }
  if (nextFragment()) { updateChrome(); return; }
  go(idx + 1);
}

function backward() {
  if (overview) { setOverview(false); return; }
  if (prevFragment()) { updateChrome(); return; }
  const prevIdx = idx - 1;
  if (prevIdx < 0) return;
  go(prevIdx);
  // Show all fragments on the previous slide when stepping back onto it.
  fragmentsOf(currentSlide()).forEach((f) => f.classList.add('visible'));
  updateChrome();
}

window.addEventListener('keydown', (e) => {
  if (e.key === 'ArrowRight' || e.key === ' ' || e.key === 'PageDown') { e.preventDefault(); forward(); }
  else if (e.key === 'ArrowLeft' || e.key === 'PageUp') { e.preventDefault(); backward(); }
  else if (e.key === 'Home') { e.preventDefault(); go(0); }
  else if (e.key === 'End') { e.preventDefault(); go(slides.length - 1); }
  else if (e.key === 'p' || e.key === 'P') { e.preventDefault(); togglePresenter(); }
  else if (e.key === 'o' || e.key === 'O') { e.preventDefault(); toggleOverview(); }
  else if (e.key === 'Escape') {
    e.preventDefault();
    if (overview) setOverview(false);
    else if (presenter) togglePresenter();
  }
});
window.addEventListener('hashchange', () => {
  const n = parseHash();
  if (n != null && n !== idx) go(n, false);
});
setInterval(() => { if (presenter) updateChrome(); }, 1000);
mount();
</script>
</body>
</html>
`
}
