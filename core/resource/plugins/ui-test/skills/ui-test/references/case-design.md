# Case design (Web UI specs)

## Principles

1. **Intent steps, checkable oracles** — write what a user means to do; assert
   what can be verified in a snapshot, URL, or screenshot — not "looks good".
2. **One behavior per scenario** — happy / edge / negative stay separate so a
   FAIL points at one cause.
3. **Seeded entry** — each scenario starts from a known URL (or documented
   preconditions). Do not rely on leftover session state unless the spec says so.
4. **Bounded** — prefer ≤8 steps per scenario; split long journeys.

## Required sections

| Section | Required | Notes |
|---------|----------|-------|
| `# Spec:` title | yes | Short stable name |
| `Target:` | yes | Absolute URL or Danmo `/raw/` / `/proxy/` URL |
| `Scope: web` | yes | Non-web → out of scope for this expert |
| `Engine:` | yes | `browser` (default) or `computer` |
| `Change-source:` | recommended | Git range, `working-tree`, or key paths (see `from-changes.md`) |
| `## Acceptance` | yes | Product-level success / forbidden states |
| `## Scenarios` | yes | At least one `### S#: …` block |

## Step verbs

| Verb | Form | Meaning |
|------|------|---------|
| `goto` | `goto /path` or full URL | Navigate (resolve relative against Target) |
| `click` | `click the "Label" button` | Resolve intent → snapshot ref → act click |
| `type` | `type "value" into the email field` | Clear+type into resolved field |
| `press` | `press Enter` | Keyboard key |
| `scroll` | `scroll down` | Viewport scroll |
| `wait` | `wait for the spinner to disappear` | Prefer condition over fixed sleep |
| `verify` | `verify the dashboard heading is visible` | Positive assertion |
| `verify not` | `verify not an error toast is shown` | Negative assertion |
| `screenshot` | `screenshot` | Save evidence (pass and fail) |

`check` / `assert` are synonyms for `verify`.

## Scenario checklist

- [ ] Happy path covered
- [ ] Empty/invalid input when forms matter
- [ ] Forbidden navigation / error state checked with `verify not`
- [ ] No real payment / irreversible delete unless goal authorizes
- [ ] Target uses `/raw/` or `/proxy/` for local apps (never `file://`)

## Naming

- Spec file: `ui-tests/specs/<area>-<feature>.md` (kebab-case)
- Scenario ids: `S1`, `S2`, … stable within the file
