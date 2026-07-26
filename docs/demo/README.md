# Product demos (bilingual)

**Video is not required.** Prefer the interactive HTML for bilingual tours (language toggle). Export GIF/MP4 only when you need README embeds or social posts.

| Asset | Use |
|-------|-----|
| [`product-tour.html`](./product-tour.html) | **Primary** — ZH/EN toggle · Architecture · Highlights · Capacity |
| `product-tour-{zh,en}.mp4` | Social / talks |
| `product-tour-{zh,en}.gif` | README embed |
| `product-tour-{zh,en}-still.png` | Social preview / link cards |
| [`work-capacity-demo.html`](./work-capacity-demo.html) | Capacity-only (legacy single-scene page) |

## Open locally

```bash
open docs/demo/product-tour.html
# or
python3 -m http.server -d docs/demo 8765
# → http://127.0.0.1:8765/product-tour.html?lang=zh
# → http://127.0.0.1:8765/product-tour.html?lang=en&tour=1
```

### Query params

| Param | Effect |
|-------|--------|
| `lang=zh\|en` | Language |
| `section=arch\|highlights\|capacity` | Start section |
| `tour=1` / `tour=fast` | Auto-advance sections (fast = shorter timings) |
| `record=1` | Tighter padding for capture |

## What’s covered

1. **Architecture** — user → LLM plans Tool Calls → Agent Loop → Turn Log → Document Stage; pillars + surface/runtime/tools/store strip  
2. **Highlights** — Document Stage · multi-agent · Turn Log · Memory/Table Store · MCP/automations · IM channels  
3. **Capacity** — research→report · delegate→slides · Feishu→sheet  

## Format guide

| Channel | Format |
|---------|--------|
| Landing / docs site | HTML (best for bilingual) |
| GitHub README | GIF or still + link to HTML |
| 即刻 / X / Reddit / Show HN | MP4 (attach) or GIF |
| GitHub Social Preview | still PNG (1280×640 crop) |
