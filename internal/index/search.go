package index

import (
	"sort"
	"strings"

	"mvn-search/internal/maven"
)

// Search filters artifacts by query keywords (space-separated, AND intersection).
// Matching is case-insensitive substring against groupId, artifactId and version.
func Search(arts []maven.Artifact, query string) []maven.Artifact {
	keywords := Keywords(query)
	if len(keywords) == 0 {
		out := make([]maven.Artifact, len(arts))
		copy(out, arts)
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].GroupID != out[j].GroupID {
				return out[i].GroupID < out[j].GroupID
			}
			if out[i].ArtifactID != out[j].ArtifactID {
				return out[i].ArtifactID < out[j].ArtifactID
			}
			return maven.CompareVersions(out[i].Version, out[j].Version) > 0
		})
		return out
	}

	hits := make([]maven.Artifact, 0, 64)
	for _, a := range arts {
		if matchesAll(a, keywords) {
			hits = append(hits, a)
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		return lessByRank(hits[i], hits[j], keywords)
	})
	return hits
}

// Keywords splits a query into non-empty lowercase keywords.
func Keywords(query string) []string {
	fields := strings.Fields(strings.ToLower(query))
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

func matchesAll(a maven.Artifact, keywords []string) bool {
	g := strings.ToLower(a.GroupID)
	id := strings.ToLower(a.ArtifactID)
	v := strings.ToLower(a.Version)
	for _, kw := range keywords {
		if !strings.Contains(g, kw) && !strings.Contains(id, kw) && !strings.Contains(v, kw) {
			return false
		}
	}
	return true
}

func lessByRank(a, b maven.Artifact, keywords []string) bool {
	ra, rb := rank(a, keywords), rank(b, keywords)
	if ra != rb {
		return ra < rb
	}
	if a.GroupID != b.GroupID {
		return a.GroupID < b.GroupID
	}
	if a.ArtifactID != b.ArtifactID {
		return a.ArtifactID < b.ArtifactID
	}
	return maven.CompareVersions(a.Version, b.Version) > 0
}

// rank scores an artifact for the query. Lower is better.
// Score = worst per-keyword rank * 10 + sum of ranks, so a strong exact
// artifactId match beats a pure groupId match even when both match all words.
func rank(a maven.Artifact, keywords []string) int {
	id := strings.ToLower(a.ArtifactID)
	g := strings.ToLower(a.GroupID)
	v := strings.ToLower(a.Version)
	worst, sum := 0, 0
	for _, kw := range keywords {
		var r int
		switch {
		case id == kw:
			r = 0
		case strings.HasPrefix(id, kw):
			r = 1
		case strings.Contains(id, kw):
			r = 2
		case strings.Contains(g, kw):
			r = 3
		case strings.Contains(v, kw):
			r = 4
		default:
			r = 5
		}
		sum += r
		if r > worst {
			worst = r
		}
	}
	return worst*10 + sum
}
