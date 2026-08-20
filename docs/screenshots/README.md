# Product screenshots

Real UI captures for README and docs. **Prefer the interactive carousel** for bilingual captions; GIF is for GitHub README embed only.

| Asset | Use |
|-------|-----|
| [`carousel.html`](./carousel.html) | **Primary** — screenshot + one-line caption, ZH/EN toggle, auto-play |
| `carousel.gif` | README embed (screenshots only; captions in HTML) |
| `carousel-still.png` | Link preview / social cards (1280px wide) |
| `ui-*.png` | Source screenshots |
| `wx2.png` | WeChat IM channel (chat) |

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

1. **Multi-agent + Plan** — Plan panel while the lead agent runs  
2. **Trajectory** — tools / thinking / specialist delegation timeline  
3. **Document Stage** — research report as Markdown (View / Edit / PDF)  
4. **Code → preview** — web game playable in Stage preview  
5. **Preview annotate** — DOM click → Composer  
6. **Expert prompts** — custom expert Prompt / Skills / Tools  
7. **Skills library** — built-in / custom / market SKILL.md  
8. **Sandbox & runtime** — turn limits, memory TopK, OS sandbox  
9. **IM channels** — WeChat channel settings  

## Extra assets (not in carousel)

| File | Content |
|------|---------|
| `ui-code-stage.png` | Session + code editor |
| `ui-memory.png` | Document Stage + Memory panel |
| `ui-env-inspect.png` | Sandbox / env introspection via `exec_shell` |
| `ui-git-panel.png` | Running turn + Git changes |
| `ui-knowledge.png` | Knowledge base Markdown |
| `ui-plugin-market.png` | Plugin market (CodeGraph) |
| `ui-connectors.png` | Connectors library |
| `ui-usage-stats.png` | Usage stats dashboard |
| `ui-web-search.png` | Web search provider settings |
| `ui-add-provider.png` | Add LLM provider modal |

## Regenerate GIF

After replacing PNGs:

```bash
# from repo root — requires ffmpeg
TMP=/tmp/carousel-build && mkdir -p "$TMP/slides"
SLIDES=(ui-team-plan ui-trajectory ui-document-stage ui-preview-game \
  ui-browser-annotate ui-expert-prompts ui-skill-editor ui-runtime-settings \
  ui-wechat-channel)
i=0; for s in "${SLIDES[@]}"; do
  ffmpeg -y -i "docs/screenshots/${s}.png" -vf "scale=1200:-1:flags=lanczos" \
    "$TMP/slides/$(printf '%02d' $i).png"; i=$((i+1)); done
ffmpeg -y -framerate 1/3.5 -i "$TMP/slides/%02d.png" \
  -vf "fps=8,scale=1200:-1:flags=lanczos,split[s0][s1];[s0]palettegen[p];[s1][p]paletteuse" \
  docs/screenshots/carousel.gif
ffmpeg -y -i docs/screenshots/ui-team-plan.png -vf "scale=1280:-1:flags=lanczos" \
  docs/screenshots/carousel-still.png
```
