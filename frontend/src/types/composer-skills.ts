import type { AvailableSkill } from '@/types'

/** Detect `@query` immediately before the caret (start of line or after whitespace). */
export function detectAtSkillQuery(
  text: string,
  caret: number,
): { start: number; query: string } | null {
  if (caret < 0 || caret > text.length) return null
  const before = text.slice(0, caret)
  const m = before.match(/(^|[\s\n])@([^\s@]*)$/)
  if (!m) return null
  const query = m[2] ?? ''
  const start = before.length - query.length - 1
  if (start < 0 || text[start] !== '@') return null
  return { start, query }
}

export function removeAtSkillQuery(text: string, start: number, caret: number): string {
  return text.slice(0, start) + text.slice(caret)
}

export function filterAvailableSkills(
  skills: AvailableSkill[],
  query: string,
): AvailableSkill[] {
  const q = query.trim().toLowerCase()
  if (!q) return skills
  return skills.filter((s) => {
    const hay = [s.id, s.name, s.description ?? '', ...(s.keywords ?? [])]
      .join('\n')
      .toLowerCase()
    return hay.includes(q)
  })
}

/** Prefix user input so the model loads the summoned skills first. */
export function buildSkillSummonPrefix(
  skills: AvailableSkill[],
  useSkillLine: (name: string) => string,
  readSkillHint: string,
): string {
  if (!skills.length) return ''
  const lines = skills.map((s) => useSkillLine(s.name || s.id))
  return `${lines.join('\n')}\n${readSkillHint}\n\n`
}

export function prependSkillSummon(
  userInput: string,
  skills: AvailableSkill[],
  useSkillLine: (name: string) => string,
  readSkillHint: string,
): string {
  const prefix = buildSkillSummonPrefix(skills, useSkillLine, readSkillHint)
  if (!prefix) return userInput
  return userInput.trim() ? `${prefix}${userInput}` : prefix.trimEnd()
}
