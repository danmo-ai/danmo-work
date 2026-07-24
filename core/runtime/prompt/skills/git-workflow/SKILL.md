---
name: git-workflow
description: Standard Git workflow operations including branching, committing, rebasing, merging, worktrees, pull requests, and finishing a branch. Use when the user mentions git operations, commits, branches, PRs, worktrees, or version control workflows.
license: MIT
compatibility: Requires git CLI and access to the project repository
metadata:
  author: danqing-teams
  version: "1.1"
  category: coding
  adapted_from: "https://github.com/obra/superpowers/tree/main/skills/using-git-worktrees"
  also_adapted_from: "https://github.com/obra/superpowers/tree/main/skills/finishing-a-development-branch"
  upstream_license: MIT
---

# Git Workflow Skill

> Worktree and branch-finish sections adapted from obra/superpowers (© Jesse Vincent / contributors, MIT); core workflow is Danmo Work original.
> Not a verbatim copy of upstream skills.

Standard Git version control operations and best practices.

## Core Workflow

### Making Changes

1. Ensure you are on the correct branch: `git branch --show-current`
2. Pull latest changes: `git pull --rebase` (or `git fetch && git rebase origin/main`)
3. Make changes using the available file tools (`read_file`, `edit`, `write`)
4. Stage changes: `git add <files>`
5. Commit with a descriptive message: use `scripts/commit.sh` for standard format

### Commit Message Convention

Follow [Conventional Commits](https://www.conventionalcommits.org/):
```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `chore`, `ci`, `build`

Examples:
- `feat(api): add skill import endpoint`
- `fix(store): handle nil skill body`
- `refactor(prompt): extract skill metadata builder`

Run `scripts/commit.sh "your message"` for interactive commit with format validation.

### Working with Branches

- Create a feature branch: `git checkout -b feat/my-feature`
- List branches: `git branch -a`
- Switch branch: `git checkout <branch>`
- Delete local branch: `git branch -d <branch>`
- Push branch: `git push -u origin <branch>`

### Branch Naming Convention

```
feat/<description>    — new features
fix/<description>     — bug fixes
refactor/<description> — code refactoring
docs/<description>    — documentation
chore/<description>   — maintenance tasks
```

### Syncing with Main

Before creating a PR:
1. `git fetch origin`
2. `git rebase origin/main`
3. Resolve conflicts if any
4. `git push --force-with-lease` (only on your feature branch)

### Pull Requests

When ready to submit changes:
1. Ensure all changes are committed and pushed
2. Verify tests pass: run the project's test command
3. Create PR with title following commit convention
4. Include a summary of changes in the PR description
5. Reference related issues with `Closes #123` or `Fixes #123`

### Parallel Work with Worktrees

Use when the user needs isolation from the current checkout (long feature, parallel tasks). Ask before creating a worktree unless they already requested one.

```bash
# Already in a linked worktree?
git rev-parse --git-dir
git rev-parse --git-common-dir
# If paths differ (and not a submodule), you are already isolated — don't nest another.

git worktree add ../repo-feat-x -b feat/x
# later:
git worktree remove ../repo-feat-x
```

Never create worktrees that fight an existing harness-managed workspace. Prefer working in place if the user declines.

### Finishing a Branch

When implementation is done:

1. **Verify** — run the project test suite; stop if failing.
2. **Detect base** — usually `main` / `master` (confirm with user if unclear).
3. **Present options** — do not silently merge:
   1. Merge locally into base
   2. Push and open a Pull Request
   3. Keep the branch as-is
   4. Discard the work (requires explicit user confirmation)
4. **Execute** the chosen option, then clean up worktrees if you created one.

## Safety Rules

- Never force-push to `main` or `master` branches
- Never commit secrets, API keys, or credentials
- Never commit large binary files without discussion
- Always pull/rebase before starting new work
- Use `git status` frequently to understand current state
- When in doubt about a git operation, ask the user for confirmation

## Common Scenarios

### Undo Last Commit (keep changes)

```
git reset --soft HEAD~1
```

### Discard Uncommitted Changes

```
git checkout -- <file>     # single file
git restore <file>         # alternative syntax
```

### Stash Changes Temporarily

```
git stash                  # save changes
git stash pop              # restore changes
```

### Squash Commits Before PR

```
git rebase -i HEAD~N       # where N is number of commits to squash
```

## Scripts

Use `scripts/commit.sh` for standardized commit creation.
