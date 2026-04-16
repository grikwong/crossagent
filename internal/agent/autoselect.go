package agent

import (
	"fmt"
	"sort"
	"strings"

	"github.com/grikwong/crossagent/internal/state"
)

// RecommendAgent returns the recommended agent name for a phase given the
// registered agent list.
//
// Heuristic (all matches are case-insensitive substring matches against
// Agent.Name and Agent.DisplayName):
//
//   - plan:      gemini -> claude -> codex -> first available
//   - review:    codex|gpt -> claude -> first available
//   - implement: claude -> codex -> first available
//   - verify:    gemini -> codex -> claude -> first available
//
// The fallback ordering preserves the current DefaultPhaseAgent mapping
// (plan/implement=claude, review/verify=codex) when no stronger
// match is present.
//
// Returns "" when agents is empty. Unknown phases return the first
// available agent name as a defensive default.
func RecommendAgent(phase string, agents []Agent) string {
	if len(agents) == 0 {
		return ""
	}

	// Stable alphabetical order for deterministic tie-breaking within a
	// preference tier.
	sorted := make([]Agent, len(agents))
	copy(sorted, agents)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	key, err := state.PhaseKey(phase)
	if err != nil {
		return sorted[0].Name
	}

	var priorities [][]string
	switch key {
	case "plan":
		priorities = [][]string{{"gemini"}, {"claude"}, {"codex"}}
	case "review":
		priorities = [][]string{{"codex", "gpt"}, {"claude"}}
	case "implement":
		priorities = [][]string{{"claude"}, {"codex"}}
	case "verify":
		priorities = [][]string{{"gemini"}, {"codex"}, {"claude"}}
	default:
		return sorted[0].Name
	}

	for _, tier := range priorities {
		if name := firstMatch(sorted, tier); name != "" {
			return name
		}
	}
	return sorted[0].Name
}

// firstMatch returns the first agent whose Name or DisplayName contains any
// of the given substrings (case-insensitive). Returns "" if none match.
func firstMatch(agents []Agent, needles []string) string {
	for _, a := range agents {
		name := strings.ToLower(a.Name)
		display := strings.ToLower(a.DisplayName)
		for _, n := range needles {
			n = strings.ToLower(n)
			if strings.Contains(name, n) || strings.Contains(display, n) {
				return a.Name
			}
		}
	}
	return ""
}

// AutoSelectAll computes the recommended agent for each phase of the given
// workflow directory. It does NOT persist the assignments — the caller is
// responsible for writing them via SetPhaseAgent. Returns a map keyed by
// phase ("plan", "review", "implement", "verify").
func AutoSelectAll(wfDir string) (map[string]string, error) {
	agents, err := ListAgents()
	if err != nil {
		return nil, err
	}
	if len(agents) == 0 {
		return nil, fmt.Errorf("no agents registered")
	}

	out := make(map[string]string, 4)
	for _, phase := range []string{"plan", "review", "implement", "verify"} {
		out[phase] = RecommendAgent(phase, agents)
	}

	// Enforce maker-checker diversity: implement must differ in family
	// from both review and verify. Re-pick the checker slot from the
	// next-best candidate of a different family when a collision is
	// detected. Implement is the anchor (maker); checkers are swapped.
	implFamily, _ := AgentFamily(out["implement"])
	if implFamily != "" {
		for _, checker := range []string{"review", "verify"} {
			curFamily, _ := AgentFamily(out[checker])
			if curFamily != implFamily {
				continue
			}
			if alt := pickDifferentFamily(checker, agents, implFamily); alt != "" {
				out[checker] = alt
			}
		}
	}

	return out, nil
}

// pickDifferentFamily returns the best agent for phase whose family
// differs from avoidFamily. It filters the agent list to candidates of
// a different family and re-runs RecommendAgent so the usual per-phase
// priority ordering is preserved. Returns "" if no candidate exists.
func pickDifferentFamily(phase string, agents []Agent, avoidFamily string) string {
	filtered := make([]Agent, 0, len(agents))
	for _, a := range agents {
		if fam, ok := AgentFamily(a.Name); ok && fam != avoidFamily {
			filtered = append(filtered, a)
		}
	}
	if len(filtered) == 0 {
		return ""
	}
	return RecommendAgent(phase, filtered)
}
