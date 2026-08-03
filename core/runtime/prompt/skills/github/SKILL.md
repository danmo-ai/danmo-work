---
name: github
description: >-
  Operate GitHub via bound MCP when configured, else gh CLI, else plain git
  (remotes/push/fetch only). Use for GitHub platform / hosting work.
license: MIT
compatibility: >-
  Builtin github MCP (mcp_github_*) when PAT/OAuth is configured;
  else GitHub CLI (gh); else git on PATH / WORK_GIT_BIN (limited).
metadata:
  author: danmo
  version: "0.4.0"
  category: coding
---

# GitHub (MCP → gh → git)

This expert owns GitHub access with a fixed degrade chain. Do not install market
GitHub connectors, ambient MCPs, or raw `curl` to `api.github.com`.

The parent turn prepends `[github-access: mcp|gh|git|none]` — **trust it**.

## Path selection

| Hint | What to use |
|------|-------------|
| `github-access: mcp` | Prefer `mcp_github_*`. On MCP failure, fall back to `gh` then `git` if the hint listed them |
| `github-access: gh` | MCP not configured — `exec_shell` → `gh …` |
| `github-access: git` | No MCP/gh — **degrade** to `exec_shell` → `git …` (hosting/plumbing only; see below) |
| `github-access: none` | Stop; report configure builtin **GitHub** connector and/or install `gh` / `git` |

Do not invent issue/PR/API state without tool evidence.

## MCP workflow (when `mcp`)

1. Call tools with full names from the tool list (`mcp_github_<tool>`).
2. Prefer read/list tools first when the goal is ambiguous.
3. On 401/403 or clear auth errors: fall back along the hint (`gh`, then `git`);
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

## `git` workflow (when `git` only)

Limited degrade — **no** GitHub Issues / PRs / Actions / Releases / search / `gh api`.

Allowed via `exec_shell` → `git`:

- Inspect remotes / upstream: `git remote -v`, `git ls-remote`, `git status`, `git branch -vv`
- Fetch / pull / push (current branch): `git fetch`, `git pull --ff-only`, `git push -u`
- Branch checkout / create for GitHub-hosted work already implied by the goal

If the goal needs Issues, PRs, checks, releases, or API: **stop in BLOCKERS** — ask for MCP PAT/OAuth or `gh` install. Do not fake PR URLs.

Still prefer parent / `git-workflow` for local commit/rebase/worktree authorship; only use `git` here when the goal is GitHub remote hosting and higher paths are unavailable.

## Safety

- **ask_user** before: merge, close, delete, release publish, secret changes,
  admin/org mutations, force-push / `--delete` — unless the goal already names that exact action.
- Prefer read-only investigation first when ambiguous.

## Anti-patterns

- Installing GitHub MCP from the market (product builtin only)
- Using a different GitHub MCP server id or `http_request` / `curl` to GitHub APIs
- Preferring `gh`/`git` when the hint says `mcp` and MCP tools are listed
- Claiming issue/PR state from `git` alone
- Installing or upgrading `gh`/`git` inside the session
