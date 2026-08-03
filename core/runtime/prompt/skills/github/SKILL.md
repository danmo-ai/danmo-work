---
name: github
description: >-
  Operate GitHub via the local gh CLI and/or the bound GitHub MCP connector
  (issues, PRs, Actions, releases, API). Use for GitHub platform work.
license: MIT
compatibility: >-
  Prefer GitHub CLI (gh) on PATH / WORK_GH_BIN with gh auth login;
  bound github MCP (mcp_github_*) as fallback when authorized.
metadata:
  author: danmo
  version: "0.2.0"
  category: coding
---

# GitHub (gh + bound MCP)

This expert owns GitHub access: local `gh` **and** the bound-only `github`
MCP connector (`mcp_github_*`). Do not reach for ambient/other GitHub MCPs
or raw `curl` to `api.github.com`.

The parent turn may prepend a `[github-gh: …]` hint — trust it.

## Path selection

| Situation | What to use |
|-----------|-------------|
| `[github-gh: ready]` | Prefer `exec_shell` → `gh …` |
| `gh` missing / not authenticated, MCP tools listed | Use `mcp_github_*` |
| MCP auth errors (401/403) but `gh` ready | Fall back to `gh` |
| Both unavailable | Stop; report install/`gh auth login` and/or connector PAT/OAuth setup |

## `gh` workflow

1. If hint is not `missing`, run once: `gh auth status`.
2. Prefer JSON: `gh <cmd> --json field1,field2`.
3. Repo scope: cwd remotes by default; `-R owner/repo` when the goal names another repo.
4. Never pass secrets on the CLI; use existing `gh` auth only.
5. Cap lists with `--limit` unless asked for a full dump.

| Intent | Starting point |
|--------|----------------|
| Issues | `gh issue list` / `gh issue view` / `gh issue create` |
| PRs | `gh pr list` / `gh pr view` / `gh pr diff` / `gh pr create` |
| Checks / CI | `gh pr checks`, `gh run list`, `gh run view <id> --log-failed` |
| Release | `gh release list` / `view` / `create` (confirm) |
| Search | `gh search issues` / `gh search prs` |
| Raw API | `gh api <path>` when no subcommand fits |

## MCP workflow

1. Call tools with full names from the tool list (`mcp_github_<tool>`).
2. Prefer read/list tools first when the goal is ambiguous.
3. On auth failure: report that the bound **GitHub** connector needs a PAT/OAuth header — do not invent tokens.

## Safety

- **ask_user** before: merge, close, delete, release publish, secret changes,
  admin/org mutations, or force/`--delete` — unless the goal already names that exact action.
- Prefer read-only investigation first when ambiguous.
- Do not `git push --force` to default branches.

## Anti-patterns

- Using a different GitHub MCP server id or `http_request` / `curl` to GitHub APIs
- Claiming issue/PR state without `gh` or MCP evidence
- Installing or upgrading `gh` inside the session
- Using `git` for hosting tasks this skill covers (`gh` / MCP first)
- Dumping unbounded `gh api` pages without limits
