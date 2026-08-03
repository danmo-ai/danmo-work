---
name: github
description: >-
  Operate GitHub via the bound GitHub MCP connector when configured, otherwise
  the local gh CLI (issues, PRs, Actions, releases, API).
license: MIT
compatibility: >-
  Builtin github MCP (mcp_github_*) when PAT/OAuth is configured;
  otherwise GitHub CLI (gh) on PATH / WORK_GH_BIN with gh auth login.
metadata:
  author: danmo
  version: "0.3.0"
  category: coding
---

# GitHub (MCP first, gh fallback)

This expert owns GitHub access: the **bound-only** `github` MCP connector and
local `gh`. Do not install market GitHub connectors, ambient MCPs, or raw
`curl` to `api.github.com`.

The parent turn prepends `[github-access: mcp|gh|none]` — **trust it**.

## Path selection

| Hint | What to use |
|------|-------------|
| `github-access: mcp` | Prefer `mcp_github_*`. On MCP auth/transport failure, fall back to `gh` if the hint mentioned a gh bin |
| `github-access: gh` | MCP not configured — use `exec_shell` → `gh …` only |
| `github-access: none` | Stop; report configure builtin **GitHub** connector (PAT/OAuth) and/or install `gh` + `gh auth login` |

Do not invent issue/PR state without tool evidence.

## MCP workflow (when `mcp`)

1. Call tools with full names from the tool list (`mcp_github_<tool>`).
2. Prefer read/list tools first when the goal is ambiguous.
3. On 401/403 or clear auth errors: if gh fallback is available, switch to `gh`;
   otherwise report that the builtin connector needs PAT/OAuth — do not invent tokens.

## `gh` workflow (when `gh`, or MCP failed)

1. Run once: `gh auth status` (unless already confirmed this turn).
2. Prefer JSON: `gh <cmd> --json field1,field2`.
3. Repo scope: cwd remotes by default; `-R owner/repo` when the goal names another repo.
4. Never pass secrets on the CLI; use existing `gh` auth only.
5. Cap lists with `--limit` unless asked for a full dump.

| Intent | `gh` starting point |
|--------|---------------------|
| Issues | `gh issue list` / `view` / `create` |
| PRs | `gh pr list` / `view` / `diff` / `create` |
| Checks / CI | `gh pr checks`, `gh run list`, `gh run view <id> --log-failed` |
| Release | `gh release list` / `view` / `create` (confirm) |
| Search | `gh search issues` / `gh search prs` |
| Raw API | `gh api <path>` when no subcommand fits |

## Safety

- **ask_user** before: merge, close, delete, release publish, secret changes,
  admin/org mutations, or force/`--delete` — unless the goal already names that exact action.
- Prefer read-only investigation first when ambiguous.

## Anti-patterns

- Installing GitHub MCP from the market (product builtin only)
- Using a different GitHub MCP server id or `http_request` / `curl` to GitHub APIs
- Preferring `gh` when the hint says `mcp` and MCP tools are listed
- Claiming issue/PR state without MCP/`gh` evidence
- Installing or upgrading `gh` inside the session
