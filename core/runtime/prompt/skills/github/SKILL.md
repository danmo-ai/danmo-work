---
name: github
description: >-
  Operate GitHub via the local gh CLI (issues, PRs, Actions, releases, API).
  Use when the user or parent agent needs GitHub platform work without a remote MCP.
license: MIT
compatibility: Requires GitHub CLI (gh) installed and authenticated (gh auth login)
metadata:
  author: danmo
  version: "0.1.0"
  category: coding
---

# GitHub (`gh` CLI)

Drive GitHub through `exec_shell` → `gh …`. There is **no** GitHub MCP in this
expert pack. The parent turn may prepend a `[github-gh: …]` hint — trust it.

## Preconditions

1. If the hint says `missing`: stop. Tell the user to install
   [GitHub CLI](https://cli.github.com/) and ensure `gh` is on `PATH`
   (or set `WORK_GH_BIN`). Do not brew/apt install yourself.
2. Otherwise run once:

   ```bash
   gh auth status
   ```

   If not logged in: stop and ask for `gh auth login` (or Enterprise host setup).
   Do not invent tokens or scrape credentials.

## Command style

- Prefer JSON: `gh <cmd> --json field1,field2` then reason over stdout.
- Repo scope: default is the git remotes of the working directory.
  Use `-R owner/repo` when the goal names another repository.
- Never pass secrets on the command line; use existing `gh` auth only.
- Avoid interactive prompts (`--confirm` / flags that skip TTY questions when safe).
- Cap list output (`--limit`) unless the goal asks for a full dump.

## Common workflows

| Intent | Starting point |
|--------|----------------|
| List / inspect issues | `gh issue list`, `gh issue view <n>` |
| Create issue | `gh issue create --title … --body …` |
| List / inspect PRs | `gh pr list`, `gh pr view <n>`, `gh pr diff` |
| Open PR | `gh pr create` (branch must be pushed; title/body from goal) |
| Checks / CI | `gh pr checks`, `gh run list`, `gh run view <id> --log-failed` |
| Release | `gh release list`, `gh release view`, `gh release create` (confirm) |
| Search | `gh search issues …`, `gh search prs …` |
| Raw API | `gh api <path>` when no first-class subcommand fits |

## Safety

- **ask_user** before: merge, close, delete, release publish, secret changes,
  admin/org mutations, or any force / `--delete` style flag — unless the goal
  already names that exact action.
- Prefer read-only investigation first when the goal is ambiguous.
- Do not `git push --force` to default branches; do not rewrite history via `gh`.

## Anti-patterns

- Calling remote GitHub MCP or `http_request` / `curl` to `api.github.com`
- Claiming issue/PR state without `gh` output
- Installing or upgrading `gh` inside the session
- Using `git` for GitHub hosting tasks this skill covers (`gh` first)
- Dumping unbounded `gh api` pages without `--paginate` awareness / limits
