# Capacity demo (Wanta-style)

Interactive product demo: **one request → Tool Calls → usable artifact**.

| File | Purpose |
|------|---------|
| [`work-capacity-demo.html`](./work-capacity-demo.html) | Self-contained animated page (open in browser) |
| [`work-capacity-demo.gif`](./work-capacity-demo.gif) | Looping capture for README / posts |
| [`work-capacity-demo.mp4`](./work-capacity-demo.mp4) | Higher-quality capture |
| [`work-capacity-demo-still.png`](./work-capacity-demo-still.png) | Still frame / social preview crop source |

## Open locally

```bash
open docs/demo/work-capacity-demo.html
# or
python3 -m http.server -d docs/demo 8765
# → http://127.0.0.1:8765/work-capacity-demo.html
```

Query params: `?scene=1|2|3`, `?record=1` (tighter padding for capture).

## Scenarios

1. **调研到报告** — `web_search` → `write_file` → Document Stage preview  
2. **委派到幻灯片** — `delegate_agent` → `memory_update` → playable slides  
3. **飞书到表格** — channel → `table_upsert` → Sheet Stage  
