package maven

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Scan walks a Maven repository root and extracts GAV coordinates.
// A version directory is recognized when it contains
// "{artifactId}-{version}.pom" or a main jar (classifier jars alone are ignored).
func Scan(repoRoot string) ([]Artifact, error) {
	root := filepath.Clean(repoRoot)
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fs.ErrNotExist
	}

	seen := make(map[string]int) // GAV -> index in out
	var out []Artifact

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// unreadable entries should not abort the whole scan
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".pom" && ext != ".jar" {
			return nil
		}
		if isMetadataName(name) {
			return nil
		}
		if ext == ".jar" && hasClassifier(name) {
			// classifier artifacts alone do not define a dependency
			return nil
		}

		versionDir := filepath.Dir(path)
		artifactDir := filepath.Dir(versionDir)
		version := filepath.Base(versionDir)
		artifactID := filepath.Base(artifactDir)
		if version == "" || version == "." || version == "/" ||
			artifactID == "" || artifactID == "." || artifactID == "/" {
			return nil
		}

		relGroup, err := filepath.Rel(root, filepath.Dir(artifactDir))
		if err != nil || relGroup == "." || relGroup == ".." || strings.HasPrefix(relGroup, "..") {
			return nil
		}
		groupID := filepath.ToSlash(relGroup)
		groupID = strings.ReplaceAll(groupID, "/", ".")

		base := strings.TrimSuffix(name, ext)
		expected := artifactID + "-" + version
		if base != expected {
			// e.g. classifier or unexpected file name; still accept pom if
			// it is exactly artifactId-version.pom (already checked).
			return nil
		}

		packaging := "pom"
		if ext == ".jar" {
			packaging = "jar"
		}

		a := Artifact{
			GroupID:    groupID,
			ArtifactID: artifactID,
			Version:    version,
			Packaging:  packaging,
		}
		key := a.GAV()
		if i, ok := seen[key]; ok {
			// prefer jar over pom when both exist
			if out[i].Packaging != "jar" && a.Packaging == "jar" {
				out[i] = a
			}
			return nil
		}
		seen[key] = len(out)
		out = append(out, a)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].GroupID != out[j].GroupID {
			return out[i].GroupID < out[j].GroupID
		}
		if out[i].ArtifactID != out[j].ArtifactID {
			return out[i].ArtifactID < out[j].ArtifactID
		}
		return CompareVersions(out[i].Version, out[j].Version) > 0
	})
	return out, nil
}

func isMetadataName(name string) bool {
	lower := strings.ToLower(name)
	switch {
	case strings.HasPrefix(lower, "maven-metadata"):
		return true
	case lower == "_remote.repositories":
		return true
	case strings.HasSuffix(lower, ".lastupdated"),
		strings.HasSuffix(lower, ".sha1"),
		strings.HasSuffix(lower, ".sha256"),
		strings.HasSuffix(lower, ".sha512"),
		strings.HasSuffix(lower, ".md5"):
		return true
	default:
		return false
	}
}

// hasClassifier reports whether a jar filename carries a classifier suffix
// (sources, javadoc, tests, linux-x86_64, ...). Filename layout is
// artifactId-version[-classifier].jar.
func hasClassifier(name string) bool {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	// artifactId may contain '-', version may contain '-', so we detect known
	// common classifiers only when they appear after the last version-looking
	// segment. Practical approach: treat well-known suffixes as classifiers.
	lower := strings.ToLower(base)
	for _, c := range []string{
		"-sources", "-javadoc", "-tests", "-test", "-test-sources",
		"-sources.jar", // defensive
	} {
		if strings.HasSuffix(lower, strings.TrimSuffix(c, ".jar")) {
			return true
		}
	}
	// platform classifiers: -linux-x86_64, -win-x86, -mac-aarch64, -native, ...
	if i := strings.LastIndex(lower, "-"); i > 0 {
		suffix := lower[i:]
		for _, p := range []string{"-native", "-linux", "-mac", "-win", "-windows", "-darwin", "-android", "-ios"} {
			if strings.HasPrefix(suffix, p) {
				return true
			}
		}
	}
	return false
}
