# Danmo Website

Official English marketing site for [Danmo](https://github.com/danmo-ai) — local-first AI for create, collaborate, and inbox.

**Live:** [https://danmo-ai.github.io/](https://danmo-ai.github.io/)

## Products

- **Make** — plugin-style image & video generation (MLX / CUDA)
- **Work** — human ↔ AI Office co-edit on a coding-agent-grade loop (docs / slides / sheets + AI Diff); multi-agent, MCP, IM
- **Inbox** — AI email information-flow organizer

## Feature tours (static HTML)

Served from `public/demos/`:

- [`/demos/office-coedit-tour.html`](https://danmo-ai.github.io/demos/office-coedit-tour.html?lang=en&tour=1) — Intent → Propose → Review → Commit
- [`/demos/product-tour.html`](https://danmo-ai.github.io/demos/product-tour.html?lang=en&tour=1) — Architecture · Highlights · Capacity

Synced from [`danmo-work/docs/demo`](https://github.com/danmo-ai/danmo-work/tree/main/docs/demo).

## Develop

```bash
npm install
npm run dev
```

## Build

```bash
npm run build
npm run preview
```

## Deploy

Pushes to `main` publish via GitHub Pages (Actions). This repo is the org site `danmo-ai.github.io`.

Built with Vue 3 + Vite + TypeScript.
