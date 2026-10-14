package tui

import (
	"strings"
	"testing"

	"mvn-search/internal/maven"
)

func arts(list ...maven.Artifact) []maven.Artifact { return list }

func TestBuildRowsSinglePackageCollapsed(t *testing.T) {
	got := buildRowsWithQuery(arts(maven.Artifact{GroupID: "g", ArtifactID: "a", Version: "1"}), "g", nil)
	if len(got) != 1 {
		t.Fatalf("want 1 collapsed package row, got %+v", got)
	}
	if got[0].kind != rowPackage || got[0].text != "g:a" || got[0].childCnt != 1 {
		t.Fatalf("bad package row: %+v", got[0])
	}
}

func TestBuildRowsEmptyQueryFlat(t *testing.T) {
	got := buildRowsWithQuery(arts(
		maven.Artifact{GroupID: "g", ArtifactID: "a", Version: "1"},
		maven.Artifact{GroupID: "g", ArtifactID: "a", Version: "2"},
	), "", nil)
	if len(got) != 2 {
		t.Fatalf("want 2 flat rows: %+v", got)
	}
}

// Nested groupId paths must hang off a shared prefix node, not sit as flat peers.
func TestBuildRowsPathPrefixNesting(t *testing.T) {
	got := buildRowsWithQuery(arts(
		maven.Artifact{GroupID: "com.illtamer.plugin", ArtifactID: "custom-vault", Version: "1"},
		maven.Artifact{GroupID: "com.illtamer.plugin", ArtifactID: "functions", Version: "1"},
		maven.Artifact{GroupID: "com.illtamer.plugin.acquisitionstore", ArtifactID: "acquisition-store", Version: "1"},
		maven.Artifact{GroupID: "com.illtamer.plugin.activityreward", ArtifactID: "activity-reward", Version: "1"},
		maven.Artifact{GroupID: "com.illtamer.lib", ArtifactID: "easy-plugin", Version: "1"},
	), "com.illta", nil)

	// Expect roughly:
	//   com.illtamer              (group)
	//     ├─ lib:easy-plugin
	//     └─ plugin
	//          ├─ custom-vault
	//          ├─ functions
	//          ├─ acquisitionstore:acquisition-store
	//          └─ activityreward:activity-reward
	var texts []string
	for _, r := range got {
		texts = append(texts, r.text)
	}
	joined := strings.Join(texts, "\n")
	if !strings.Contains(joined, "com.illtamer") {
		t.Fatalf("missing folded root prefix: %s", joined)
	}
	if !strings.Contains(joined, "plugin") {
		t.Fatalf("missing plugin node: %s", joined)
	}

	// plugin's children must include custom-vault AND acquisitionstore under it
	pluginIdx := -1
	for i, r := range got {
		if r.kind == rowGroup && strings.Contains(r.text, "plugin") && !strings.Contains(r.text, "acquisition") {
			pluginIdx = i
			break
		}
	}
	if pluginIdx < 0 {
		t.Fatalf("no plugin group node: %+v", got)
	}
	pluginKey := got[pluginIdx].key
	var childTexts []string
	for _, r := range got {
		if r.parent == pluginKey {
			childTexts = append(childTexts, r.text)
		}
	}
	joinedChildren := strings.Join(childTexts, " | ")
	for _, want := range []string{"custom-vault", "functions", "acquisitionstore", "activityreward"} {
		if !strings.Contains(joinedChildren, want) {
			t.Fatalf("plugin children missing %q: %s", want, joinedChildren)
		}
	}

	// acquisitionstore must NOT be a root-level peer of com.illtamer
	for _, r := range got {
		if r.depth == 0 && strings.Contains(r.text, "acquisitionstore") {
			t.Fatalf("acquisitionstore should nest under plugin, not root: %+v", r)
		}
	}
}

func TestBuildRowsExpandPackageShowsVersions(t *testing.T) {
	results := arts(
		maven.Artifact{GroupID: "com.a", ArtifactID: "x", Version: "2"},
		maven.Artifact{GroupID: "com.a", ArtifactID: "x", Version: "1"},
		maven.Artifact{GroupID: "com.a", ArtifactID: "y", Version: "1"},
	)
	// expand package com.a:x — key is ga
	expanded := expandState{"com.a:x": true}
	got := buildRowsWithQuery(results, "com", expanded)
	foundVer := false
	for _, r := range got {
		if r.kind == rowLeaf && r.text == "2" {
			foundVer = true
		}
	}
	if !foundVer {
		t.Fatalf("expanded versions missing: %+v", got)
	}
}

func TestBuildRowsCollapseGroup(t *testing.T) {
	results := arts(
		maven.Artifact{GroupID: "com.a", ArtifactID: "x", Version: "1"},
		maven.Artifact{GroupID: "com.a", ArtifactID: "y", Version: "1"},
	)
	// collapse the folded path node (key is the folded path "com.a")
	expanded := expandState{"com.a": false}
	got := buildRowsWithQuery(results, "com", expanded)
	if len(got) != 1 || got[0].kind != rowGroup || got[0].expanded {
		t.Fatalf("collapsed group only: %+v", got)
	}
}

func TestDecorateBranches(t *testing.T) {
	got := buildRowsWithQuery(arts(
		maven.Artifact{GroupID: "com.a", ArtifactID: "x", Version: "3"},
		maven.Artifact{GroupID: "com.a", ArtifactID: "x", Version: "2"},
		maven.Artifact{GroupID: "com.a", ArtifactID: "x", Version: "1"},
		maven.Artifact{GroupID: "com.a", ArtifactID: "y", Version: "1"},
	), "com", expandState{"com.a:x": true})

	var sawMid, sawLast bool
	for _, r := range got {
		if r.kind == rowLeaf && strings.Contains(r.branch, "├─") {
			sawMid = true
		}
		if r.kind == rowLeaf && strings.Contains(r.branch, "└─") {
			sawLast = true
		}
	}
	if !sawMid || !sawLast {
		t.Fatalf("want mixed ├─/└─ leaves: %+v", got)
	}
}
