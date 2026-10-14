package maven

import (
	"fmt"
	"sort"
	"strings"
)

// Artifact is a Maven coordinate (GAV) discovered in a local repository.
type Artifact struct {
	GroupID    string `json:"group_id"`
	ArtifactID string `json:"artifact_id"`
	Version    string `json:"version"`
	Packaging  string `json:"packaging"`
}

// GA returns "groupId:artifactId".
func (a Artifact) GA() string {
	return a.GroupID + ":" + a.ArtifactID
}

// GAV returns "groupId:artifactId:version".
func (a Artifact) GAV() string {
	return a.GroupID + ":" + a.ArtifactID + ":" + a.Version
}

// FormatMaven renders a Maven <dependency> snippet.
func (a Artifact) FormatMaven() string {
	return fmt.Sprintf(
		"<dependency>\n    <groupId>%s</groupId>\n    <artifactId>%s</artifactId>\n    <version>%s</version>\n</dependency>",
		a.GroupID, a.ArtifactID, a.Version,
	)
}

// FormatGradle renders a Gradle (Groovy DSL) dependency line.
func (a Artifact) FormatGradle() string {
	return fmt.Sprintf("implementation '%s'", a.GAV())
}

// CompareVersions compares two Maven-ish version strings.
// Numeric segments compare numerically; non-numeric segments lexicographically.
// A missing numeric segment sorts before a present one (1.0 < 1.0.1).
// Result: -1 if a < b, 0 if equal, 1 if a > b.
func CompareVersions(a, b string) int {
	if a == b {
		return 0
	}
	at, bt := splitVersion(a), splitVersion(b)
	n := len(at)
	if len(bt) > n {
		n = len(bt)
	}
	for i := 0; i < n; i++ {
		var as, bs string
		if i < len(at) {
			as = at[i]
		}
		if i < len(bt) {
			bs = bt[i]
		}
		if as == bs {
			continue
		}
		if as == "" {
			// trailing qualifier makes a version pre-release: 1.0 > 1.0-SNAPSHOT
			if _, ok := parseNum(bs); ok {
				return -1
			}
			return 1
		}
		if bs == "" {
			if _, ok := parseNum(as); ok {
				return 1
			}
			return -1
		}
		an, aNum := parseNum(as)
		bn, bNum := parseNum(bs)
		if aNum && bNum {
			if an != bn {
				if an < bn {
					return -1
				}
				return 1
			}
			continue
		}
		// numeric tokens sort before non-numeric (1.0 < 1.0-alpha is uncommon;
		// prefer treating pure numbers as greater stability: 1.0 > 1.0-SNAPSHOT)
		if aNum != bNum {
			if aNum {
				return 1
			}
			return -1
		}
		if as < bs {
			return -1
		}
		return 1
	}
	return 0
}

// splitVersion splits a version into comparable tokens on common separators.
// "1.0.1" → ["1","0","1"]; "33.5.0-jre" → ["33","5","0","jre"].
func splitVersion(v string) []string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	var (
		out []string
		sb  strings.Builder
	)
	flush := func() {
		if sb.Len() > 0 {
			out = append(out, sb.String())
			sb.Reset()
		}
	}
	for _, r := range v {
		if r == '.' || r == '-' || r == '_' || r == '+' {
			flush()
			continue
		}
		sb.WriteRune(r)
	}
	flush()
	return out
}

func parseNum(s string) (int64, bool) {
	if s == "" {
		return 0, false
	}
	var n int64
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int64(r-'0')
	}
	return n, true
}

// SortByGAVersion sorts artifacts by group, artifact, then version descending.
func SortByGAVersion(list []Artifact) {
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i], list[j]
		if a.GroupID != b.GroupID {
			return a.GroupID < b.GroupID
		}
		if a.ArtifactID != b.ArtifactID {
			return a.ArtifactID < b.ArtifactID
		}
		return CompareVersions(a.Version, b.Version) > 0
	})
}
