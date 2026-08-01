# Apply to danmo-ai.github.io

Cloud agent could not push to `danmo-ai/danmo-ai.github.io` (403). Apply from this PR:

| Source in `danmo-work` | Destination in `danmo-ai.github.io` |
|------------------------|-------------------------------------|
| `docs/website-sync/src/views/TeamsView.vue` | `src/views/TeamsView.vue` |
| `docs/website-sync/src/views/HomeView.vue` | `src/views/HomeView.vue` |
| `docs/demo/office-coedit-tour.html` | `public/demos/office-coedit-tour.html` |
| `docs/demo/product-tour.html` | `public/demos/product-tour.html` |
| `docs/screenshots/ui-office-coedit.png` | `public/screenshots/ui-office-coedit.png` |
| `docs/screenshots/ui-ai-doc-modify.png` | `public/screenshots/ui-ai-doc-modify.png` |
| `docs/screenshots/ui-team-intent.png` | `public/screenshots/ui-team-intent.png` |
| `docs/screenshots/ui-team-delegate.png` | `public/screenshots/ui-team-delegate.png` |
| `docs/website-sync/README.site.md` | `README.md` (merge carefully; site also has `README.zh-CN.md`) |

```bash
# from a checkout of danmo-ai.github.io, with danmo-work as sibling
# (repo path may be ../website or ../danmo-ai.github.io):
cp ../DanQing-Teams/docs/website-sync/src/views/TeamsView.vue src/views/
cp ../DanQing-Teams/docs/website-sync/src/views/HomeView.vue src/views/
mkdir -p public/demos public/screenshots
cp ../DanQing-Teams/docs/demo/office-coedit-tour.html public/demos/
cp ../DanQing-Teams/docs/demo/product-tour.html public/demos/
cp ../DanQing-Teams/docs/screenshots/ui-office-coedit.png public/screenshots/
cp ../DanQing-Teams/docs/screenshots/ui-ai-doc-modify.png public/screenshots/
cp ../DanQing-Teams/docs/screenshots/ui-team-intent.png public/screenshots/
cp ../DanQing-Teams/docs/screenshots/ui-team-delegate.png public/screenshots/
# optional: merge README.site.md into README.md (do not clobber ZH README)
```

Suggested commit message: `feat(work): Office co-edit + LLM-driven expert team screenshots`

After Pages deploy:

- https://danmo-ai.github.io/work
- https://danmo-ai.github.io/demos/office-coedit-tour.html?lang=en&tour=1
