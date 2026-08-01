# Danmo Website

[English](README.md) · [中文](README.zh-CN.md)

Marketing site for [Danmo](https://github.com/danmo-ai) — local-first AI for create, collaborate, and inbox.

**Site:** https://danmo-ai.github.io/

## Stack

Vue 3 · Vite · TypeScript · Vue Router

Published as the org GitHub Pages site (`danmo-ai.github.io`).

## Pages

| Path | Product | Focus |
| --- | --- | --- |
| `/` | Home | Suite overview |
| `/make` | Make | Local image & video generation (MLX / CUDA) |
| `/work` | Work | Human ↔ AI Office co-edit; LLM-driven expert team |
| `/inbox` | Inbox | AI email as information flow |

Legacy routes `/studio`, `/teams`, `/mail` redirect to the paths above.

## Feature tours (static HTML)

Served from `public/demos/`:

- [`/demos/office-coedit-tour.html`](https://danmo-ai.github.io/demos/office-coedit-tour.html?lang=en&tour=1) — Intent → Propose → Review → Commit
- [`/demos/product-tour.html`](https://danmo-ai.github.io/demos/product-tour.html?lang=en&tour=1) — Architecture · Highlights · Capacity

Synced from [`danmo-work/docs/demo`](https://github.com/danmo-ai/danmo-work/tree/main/docs/demo) and screenshots under `docs/screenshots/`.

## Quick start

Requires Node.js 22+.

```bash
npm install
npm run dev
```

| Command | Action |
| --- | --- |
| `npm run dev` | Local dev server |
| `npm run build` | Type-check, build to `dist/`, write `404.html` for SPA fallback |
| `npm run preview` | Preview the production build |

## Deploy

Push to `main` (or run the workflow manually). CI builds and deploys via [GitHub Actions](.github/workflows/deploy.yml).

## Layout

```
src/
  components/   Header, footer, product hero
  views/        Page views
  router/       Routes & redirects
  styles/       Global CSS
public/         Favicon, icons, screenshots, demos
brand/          Brand SVGs
```
