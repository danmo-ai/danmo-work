---
name: mcp-connectors
source: builtin
description: Use product connectors (MCP under the hood) and auth gateways without stuffing every action schema into context.
---

# Connectors

Danmo Work mounts enabled **connectors** into the Agent Loop. Each action is
exposed as a tool named `mcp_<connector>_<action>` (see system
`<mcp-tool-naming>`). Skills may cite short names; always call the full `mcp_*`
name from the tool list.

## When to use what

- Temporary / one-off HTTP → `http_request`
- Product SaaS actions (Notion, GitHub, Feishu docs) → connector actions
- Auth-heavy SaaS with many APIs → prefer a gateway connector (Composio /
  OpenConnector) instead of many raw HTTP calls
- Chat ingress (WeChat / Feishu / WeCom / QQ) is **not** a connector — it only
  delivers messages

## Gateway meta-tools (Composio-style)

If the mounted connector exposes search / connect / execute meta-actions:

1. Search for the right upstream action
2. Connect / wait for OAuth if the user is not linked yet (`ask_user` with the
   authorize link)
3. Execute; prefer batch execute when available
4. Do **not** dump secrets or tokens into replies

## Auth

If a connector returns unauthorized / 401:

1. Ask the user to open **Connectors** in the UI and complete auth (headers or
   OAuth token) for that connector.
2. Do not invent tokens or put secrets in `http_request` unless the user
   explicitly pasted a short-lived token for a one-off call.

## Permissions

- Connector actions are `external` risk — expect approval unless permission mode
  allows them; dangerous shell and external connector actions may still ask.
- Never put API keys into tool arguments when a secret header / OAuth token
  already exists on the connector.
