package service

import (
	"regexp"
	"strings"
)

var (
	reSkillBodyBacktick = regexp.MustCompile("`([^`]+)`")
	reSkillBodyMdLink   = regexp.MustCompile(`\(([^)\s]+)\)`)
)

// NormalizeSkillBodyRefs prefixes bare skill resource refs in a SKILL.md body
// with skillID (the skill meta id used by read_skill / <available_skills><path>).
//
// Rewrites only conservative targets in backticks and Markdown link destinations:
//
//	`references/foo.md`  → `skillID/references/foo.md`
//	(scripts/run.sh)     → (skillID/scripts/run.sh)
//
// Leaves alone: URLs (://), absolute paths, paths already prefixed with skillID,
// and paths already prefixed with another id (e.g. other/references/…).
// Directory-only mentions like `references/` are not rewritten.
func NormalizeSkillBodyRefs(body, skillID string) string {
	return normalizeSkillBodyRefs(body, skillID, "")
}

// NormalizeSkillBodyRefsAfterIDChange is like NormalizeSkillBodyRefs, but also
// remaps previousID/(scripts|references|assets)/… → skillID/… when the skill
// meta id was overridden after import (e.g. market catalog id).
func NormalizeSkillBodyRefsAfterIDChange(body, skillID, previousID string) string {
	return normalizeSkillBodyRefs(body, skillID, previousID)
}

func normalizeSkillBodyRefs(body, skillID, previousID string) string {
	if body == "" || skillID == "" {
		return body
	}
	body = reSkillBodyBacktick.ReplaceAllStringFunc(body, func(m string) string {
		inner := m[1 : len(m)-1]
		if next, ok := rewriteSkillRef(inner, skillID, previousID); ok {
			return "`" + next + "`"
		}
		return m
	})
	body = reSkillBodyMdLink.ReplaceAllStringFunc(body, func(m string) string {
		inner := m[1 : len(m)-1]
		if next, ok := rewriteSkillRef(inner, skillID, previousID); ok {
			return "(" + next + ")"
		}
		return m
	})
	return body
}

func rewriteSkillRef(raw, skillID, previousID string) (string, bool) {
	p := strings.TrimSpace(raw)
	if p == "" || strings.Contains(p, "://") || strings.HasPrefix(p, "/") {
		return "", false
	}
	p = strings.TrimPrefix(p, "./")

	if previousID != "" && previousID != skillID {
		pref := previousID + "/"
		if strings.HasPrefix(p, pref) {
			rest := p[len(pref):]
			if isBareSkillResourcePath(rest) {
				return skillID + "/" + rest, true
			}
		}
	}

	if strings.HasPrefix(p, skillID+"/") {
		return "", false
	}

	if isBareSkillResourcePath(p) {
		return skillID + "/" + p, true
	}
	return "", false
}

// isBareSkillResourcePath reports whether p is a skill-relative resource path
// under scripts/, references/, or assets/ with a non-empty file segment.
func isBareSkillResourcePath(p string) bool {
	for _, prefix := range ValidSkillResourcePrefixes {
		if !strings.HasPrefix(p, prefix) {
			continue
		}
		rest := p[len(prefix):]
		if rest == "" || rest == "." {
			return false
		}
		return true
	}
	return false
}
