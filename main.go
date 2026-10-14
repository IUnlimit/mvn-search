package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"mvn-search/internal/index"
	"mvn-search/internal/tui"
)

func main() {
	repo := flag.String("repo", index.DefaultRepo(), "maven local repository root")
	rebuild := flag.Bool("rebuild", false, "force rebuild the index cache")
	flag.Parse()

	repoRoot, err := filepath.Abs(*repo)
	if err != nil {
		fatal(err)
	}
	if st, err := os.Stat(repoRoot); err != nil || !st.IsDir() {
		fatal(fmt.Errorf("repository not found: %s", repoRoot))
	}

	cachePath := index.CachePath()
	idx, built, err := index.LoadOrBuild(repoRoot, cachePath, *rebuild)
	if err != nil {
		fatal(err)
	}
	if built {
		fmt.Fprintf(os.Stderr, "indexed %d artifacts from %s (cache: %s)\n", len(idx.Artifacts), repoRoot, cachePath)
	}

	p := tea.NewProgram(tui.New(idx))
	if _, err := p.Run(); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "mvn-search:", err)
	os.Exit(1)
}
