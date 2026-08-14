---
name: skill-creator
source: builtin
description: Create new skills, modify and improve existing skills for Danmo Work agents. Use when users want to create a skill from scratch, edit or optimize an existing skill, or understand the Agentskills specification format.
license: Apache-2.0
compatibility: Requires Danmo Work 1.0+
metadata:
  author: danmo-work
  version: "1.0"
  category: general
  based_on: anthropics/skills skill-creator
---

# Skill Creator

A skill for creating and improving skills that extend agent capabilities in Danmo Work.

## What Is a Skill?

A skill is a directory containing a `SKILL.md` file with YAML frontmatter (metadata) and Markdown instructions. Skills teach agents how to perform specialized tasks in a repeatable way.

### Directory Structure

```
skill-name/
├── SKILL.md          # Required: metadata + instructions
├── scripts/          # Optional: executable code the agent can run
├── references/       # Optional: documentation loaded on demand
└── assets/           # Optional: templates, images, data files
```

### Where to put custom skills

Place a skill directory (with `SKILL.md`) under one of these roots. They are scanned on every new turn and merged into `<available_skills>` without writing to the database:

| Path | Scope |
|------|-------|
| `~/.agents/skills/` | User |
| `~/.danmo-work/skills/` | User |
| `<projectRoot>/.agents/skills/` | Project |
| `<projectRoot>/.danmo-work/skills/` | Project |

Later paths override the same skill ID. Prefer project `.danmo-work/skills/` for repo-specific workflows.

### SKILL.md Format

```yaml
---
name: skill-name          # Required: lowercase, hyphens, 1-64 chars, must match dir
source: builtin
description: ...          # Required: 1-1024 chars, what + when to use
license: MIT              # Optional
compatibility: ...        # Optional: environment requirements
metadata:                 # Optional: key-value map
  author: team
  version: "1.0"
allowed-tools: "Bash Read" # Optional (Experimental): pre-approved tools
---

# Skill Title

Markdown instructions for the agent...
```

## Creating a Skill

### Step 1: Understand the Intent

Ask the user:
1. What should this skill enable the agent to do?
2. When should it be triggered? (what user phrases or contexts)
3. What's the expected output or workflow?
4. Does it need scripts, references, or assets?

### Step 2: Write the SKILL.md

- **name**: Short, lowercase, hyphenated identifier matching the directory name.
- **description**: Include BOTH what the skill does AND when to use it. Be specific — this is the primary triggering mechanism. Make it slightly "pushy" to avoid under-triggering.
- **Body**: Write clear, actionable instructions. Use imperative form. Include:
  - Step-by-step workflow
  - Example inputs and outputs
  - Common edge cases and how to handle them
  - References to bundled scripts/references with clear usage guidance

### Step 3: Structure for Progressive Disclosure

Skills use three-level loading:
1. **Metadata** (~100 words): `name` + `description` — always loaded
2. **Body**: Full SKILL.md — loaded when skill activates (<500 lines recommended)
3. **Resources**: `scripts/`, `references/`, `assets/` — loaded on demand

Keep SKILL.md under 500 lines. Move detailed reference material to `references/` files.

### Step 4: Bundle Resources

- **scripts/**: Self-contained executable code. Document dependencies clearly.
- **references/**: Additional docs loaded when needed. Keep files focused (one topic per file). Include a table of contents for files >300 lines.
- **assets/**: Templates, images, data files used in output.

## Guidelines

- Explain **why** things are important, not just what to do. Agents understand reasoning.
- Prefer imperative form in instructions.
- Include concrete examples with realistic inputs and expected outputs.
- Test your skill by asking an agent to perform the task and observing results.
- For skills with objectively verifiable outputs, write test cases.
- Do not include malware, exploit code, or content that could compromise security.

## Best Practices

- **Description quality**: The description is the most important field for triggering. Include specific keywords and contexts. Bad: "Helps with PDFs." Good: "Extract text and tables from PDF files, fill PDF forms, merge PDFs. Use when working with PDF documents or when the user mentions PDFs, forms, or document extraction."
- **File organization**: If a skill supports multiple domains/frameworks, organize `references/` by variant (e.g., `aws.md`, `gcp.md`, `azure.md`).
- **Lean instructions**: Remove instructions that aren't contributing value. Watch for repeated work patterns across test cases and extract those into `scripts/`.
