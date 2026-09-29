package subscription

import (
	"fmt"
	"strings"
)

// Export names remain readable. Only collisions (including reserved policies)
// receive a small suffix, and generated suffixes never consume another name.
func nodeLabels(nodes []Node, ini bool) []string {
	bases := make([]string, len(nodes))
	originals := map[string]bool{}
	for i, n := range nodes {
		bases[i] = strings.TrimSpace(n.Name)
		if ini {
			bases[i] = iniLabel(n)
		}
		originals[strings.ToLower(bases[i])] = true
	}
	used := map[string]bool{"direct": true, "reject": true, "reject-drop": true, "pass": true, "global": true, "xingdu": true}
	out := make([]string, len(nodes))
	for i, base := range bases {
		label := base
		if used[strings.ToLower(label)] {
			for suffix := 2; ; suffix++ {
				label = fmt.Sprintf("%s - %d", base, suffix)
				if !originals[strings.ToLower(label)] && !used[strings.ToLower(label)] {
					break
				}
			}
		}
		used[strings.ToLower(label)] = true
		out[i] = label
	}
	return out
}
