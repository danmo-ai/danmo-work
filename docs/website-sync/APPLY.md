# Apply to danmo-ai.github.io

Cloud agent could not push to `danmo-ai/danmo-ai.github.io` (403). Apply from this PR:

| Source in `danmo-work` | Destination in `danmo-ai.github.io` |
|------------------------|-------------------------------------|
| `docs/website-sync/src/views/TeamsView.vue` | `src/views/TeamsView.vue` |
| `docs/website-sync/src/views/HomeView.vue` | `src/views/HomeView.vue` |
| `docs/demo/office-coedit-tour.html` | `public/demos/office-coedit-tour.html` |
| `docs/demo/product-tour.html` | `public/demos/product-tour.html` |
| `docs/website-sync/README.site.md` | `README.md` (optional) |

```bash
# from a checkout of danmo-ai.github.io, with danmo-work as sibling:
cp ../danmo-work/docs/website-sync/src/views/TeamsView.vue src/views/
cp ../danmo-work/docs/website-sync/src/views/HomeView.vue src/views/
mkdir -p public/demos
cp ../danmo-work/docs/demo/office-coedit-tour.html public/demos/
cp ../danmo-work/docs/demo/product-tour.html public/demos/
```

Suggested commit message: `feat(work): spotlight human↔AI Office co-edit with feature tour`

After Pages deploy:

- https://danmo-ai.github.io/work
- https://danmo-ai.github.io/demos/office-coedit-tour.html?lang=en&tour=1
