---
name: mcp-connectors
description: Use MCP connectors and auth gateways (Composio / OpenConnector) without stuffing every tool schema into context.
---

# MCP connectors

Danmo Work mounts enabled MCP tools into the Agent Loop as `mcp_<server>_<tool>`.

## When to use

- Product SaaS actions (Notion, GitHub, Feishu docs) → MCP tools
- One-off REST exploration → `http_request`
- Chat ingress (WeChat / Feishu / WeCom / QQ) is **not** a connector — it only delivers messages

## Gateway meta-tools (Composio-style)

If the mounted server exposes search / connect / execute meta-tools:

1. Search for the right upstream action
2. Connect / wait for OAuth if the user is not linked yet (`ask_user` with the authorize link)
3. Execute; prefer batch execute when available
4. Do **not** dump secrets or tokens into replies

## Safety

- MCP tools are `external` risk — expect approval unless permission mode is `auto` (external still asks by default for MCP)
- Never put API keys into tool arguments when a secret header / OAuth token already exists on the server
