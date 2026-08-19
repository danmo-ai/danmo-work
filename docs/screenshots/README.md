# Product screenshots

Real UI captures for README and docs. **Prefer the interactive carousel** for bilingual captions; GIF is for GitHub README embed only.

| Asset | Use |
|-------|-----|
| [`carousel.html`](./carousel.html) | **Primary** — screenshot + one-line caption, ZH/EN toggle, auto-play |
| `carousel.gif` | README embed (screenshots only; captions in HTML) |
| `carousel-still.png` | Link preview / social cards (1280px wide) |
| `ui-*.png` | Source screenshots (3456×2168) |
| `wx1.png` | WeChat IM channel |

## Open locally

```bash
open docs/screenshots/carousel.html
# or
python3 -m http.server -d docs/screenshots 8765
# → http://127.0.0.1:8765/carousel.html?lang=zh
# → http://127.0.0.1:8765/carousel.html?lang=en&fast=1
```

### Query params

| Param | Effect |
|-------|--------|
| `lang=zh\|en` | Caption language |
| `autoplay=0` | Start paused |
| `fast=1` | 3.5s per slide (default 5s) |

## Slides (9)

1. **Multi-agent** — delegation + Plan panel  
2. **Research → report** — built-in browser preview  
3. **Document Stage** — doc / slides / sheet  
4. **AI inline edit** — polish / expand / modify  
5. **Browser annotate** — DOM click → Composer  
6. **Code → preview** — snake game demo  
7. **Skills library** — built-in / custom / market  
8. **Sandbox & runtime** — OS sandbox, network deny  
9. **IM channels** — WeChat same loop  

## Regenerate GIF

After replacing PNGs:

```bash
# from repo root — requires ffmpeg
TMP=/tmp/carousel-build && mkdir -p "$TMP/slides"
SLIDES=(ui-team-delegate ui-market-report ui-office-coedit ui-ai-doc-modify \
  ui-browser-annotate ui-snake-game ui-skill-editor ui-runtime-settings wx1)
i=0; for s in "${SLIDES[@]}"; do
  ffmpeg -y -i "docs/screenshots/${s}.png" -vf "scale=1200:-1:flags=lanczos" \
    "$TMP/slides/$(printf '%02d' $i).png"; i=$((i+1)); done
ffmpeg -y -framerate 1/3.5 -i "$TMP/slides/%02d.png" \
  -vf "fps=8,scale=1200:-1:flags=lanczos,split[s0][s1];[s0]palettegen[p];[s1][p]paletteuse" \
  docs/screenshots/carousel.gif
```
