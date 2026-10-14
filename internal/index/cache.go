package index

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"mvn-search/internal/maven"
)

const schemaVersion = 1

// Index is a cached snapshot of local Maven repository coordinates.
type Index struct {
	SchemaVersion int              `json:"schema_version"`
	Repo          string           `json:"repo"`
	BuiltAt       time.Time        `json:"built_at"`
	Fingerprint   string           `json:"fingerprint"`
	Artifacts     []maven.Artifact `json:"artifacts"`
}

// CacheDir returns the XDG cache directory for this tool.
func CacheDir() string {
	if x := os.Getenv("XDG_CACHE_HOME"); x != "" {
		return filepath.Join(x, "mvn-search")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".cache", "mvn-search")
	}
	return filepath.Join(home, ".cache", "mvn-search")
}

// CachePath returns the index cache file path.
func CachePath() string {
	return filepath.Join(CacheDir(), "index.json")
}

// DefaultRepo returns the default Maven local repository path.
func DefaultRepo() string {
	if x := os.Getenv("MVN_SEARCH_REPO"); x != "" {
		return x
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".m2/repository"
	}
	return filepath.Join(home, ".m2", "repository")
}

// Build scans repoRoot and creates a fresh index.
func Build(repoRoot string) (*Index, error) {
	arts, err := maven.Scan(repoRoot)
	if err != nil {
		return nil, err
	}
	return &Index{
		SchemaVersion: schemaVersion,
		Repo:          repoRoot,
		BuiltAt:       time.Now(),
		Fingerprint:   fingerprint(repoRoot, arts),
		Artifacts:     arts,
	}, nil
}

// Load reads the cache file. It returns an error when missing or incompatible.
func Load(path, repoRoot string) (*Index, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parse index cache: %w", err)
	}
	if idx.SchemaVersion != schemaVersion {
		return nil, errors.New("index cache schema mismatch")
	}
	if idx.Repo != repoRoot {
		return nil, errors.New("index cache belongs to another repository")
	}
	return &idx, nil
}

// Save writes the index cache atomically.
func Save(path string, idx *Index) error {
	if idx == nil {
		return errors.New("nil index")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadOrBuild loads a valid cache, or rebuilds and persists one.
func LoadOrBuild(repoRoot, cachePath string, force bool) (*Index, bool, error) {
	if !force {
		if idx, err := Load(cachePath, repoRoot); err == nil && len(idx.Artifacts) > 0 {
			return idx, false, nil
		}
	}
	idx, err := Build(repoRoot)
	if err != nil {
		return nil, false, err
	}
	// cache write failures are non-fatal for UX
	_ = Save(cachePath, idx)
	return idx, true, nil
}

func fingerprint(repoRoot string, arts []maven.Artifact) string {
	last := ""
	if n := len(arts); n > 0 {
		last = arts[n-1].GAV()
	}
	return fmt.Sprintf("%s|%d|%s", repoRoot, len(arts), last)
}
