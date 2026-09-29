package machine

import (
	"regexp"
	"strings"
)

// MinimumDeploymentVersion is the oldest Agent that supports the deployment contract.
const MinimumDeploymentVersion = "0.7.0-dev"

var semanticVersion = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)

func numeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return s != ""
}
func numberCompare(a, b string) int {
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return strings.Compare(a, b)
}
func parseVersion(s string) []string {
	if len(s) > 64 {
		return nil
	}
	p := semanticVersion.FindStringSubmatch(s)
	if p == nil {
		return nil
	}
	for _, part := range strings.Split(p[4], ".") {
		if numeric(part) && len(part) > 1 && part[0] == '0' {
			return nil
		}
	}
	return p
}

// VersionAtLeast compares semantic versions, ignoring build metadata.
func VersionAtLeast(actual, minimum string) bool {
	a, b := parseVersion(actual), parseVersion(minimum)
	if a == nil || b == nil {
		return false
	}
	for i := 1; i <= 3; i++ {
		if n := numberCompare(a[i], b[i]); n != 0 {
			return n > 0
		}
	}
	if a[4] == b[4] || a[4] == "" {
		return true
	}
	if b[4] == "" {
		return false
	}
	ap, bp := strings.Split(a[4], "."), strings.Split(b[4], ".")
	for i := 0; i < len(ap) && i < len(bp); i++ {
		x, y := ap[i], bp[i]
		if x == y {
			continue
		}
		xn, yn := numeric(x), numeric(y)
		if xn && yn {
			return numberCompare(x, y) > 0
		}
		if xn != yn {
			return !xn
		}
		return x > y
	}
	return len(ap) >= len(bp)
}
