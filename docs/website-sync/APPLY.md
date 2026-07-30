# Transfer to danmo-ai.github.io workspace

This cloud agent run is bound to `danmo-work` only and **cannot push** to
`danmo-ai/danmo-ai.github.io` (403). Use one of the methods below from a
workspace / agent that has write access to the website repo.

## Option A — apply patch (recommended)

In a `danmo-ai.github.io` checkout (or website Cloud Agent):

```bash
git checkout -b cursor/work-office-coedit-f063
# if danmo-work is a sibling checkout:
git apply ../danmo-work/docs/website-sync/website-coedit.patch
# or download from the PR:
# curl -L https://raw.githubusercontent.com/danmo-ai/danmo-work/cursor/office-coedit-product-intro-f063/docs/website-sync/website-coedit.patch | git apply

git add -A
git commit -m "feat(work): spotlight human↔AI Office co-edit with feature tour"
git push -u origin cursor/work-office-coedit-f063
# then open PR → main
```

## Option B — copy files

| Source in `danmo-work` | Destination in `danmo-ai.github.io` |
|------------------------|-------------------------------------|
| `docs/website-sync/src/views/TeamsView.vue` | `src/views/TeamsView.vue` |
| `docs/website-sync/src/views/HomeView.vue` | `src/views/HomeView.vue` |
| `docs/demo/office-coedit-tour.html` | `public/demos/office-coedit-tour.html` |
| `docs/demo/product-tour.html` | `public/demos/product-tour.html` |
| `docs/website-sync/README.site.md` | `README.md` (optional) |

```bash
cp ../danmo-work/docs/website-sync/src/views/TeamsView.vue src/views/
cp ../danmo-work/docs/website-sync/src/views/HomeView.vue src/views/
mkdir -p public/demos
cp ../danmo-work/docs/demo/office-coedit-tour.html public/demos/
cp ../danmo-work/docs/demo/product-tour.html public/demos/
```

## Option C — git bundle

```bash
git fetch ../danmo-work/docs/website-sync/website-coedit.bundle cursor/work-office-coedit-f063:cursor/work-office-coedit-f063
git checkout cursor/work-office-coedit-f063
git push -u origin cursor/work-office-coedit-f063
```

After Pages deploy:

- https://danmo-ai.github.io/work
- https://danmo-ai.github.io/demos/office-coedit-tour.html?lang=en&tour=1
