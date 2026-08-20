import type { AvailableSkill } from '@/types'

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
