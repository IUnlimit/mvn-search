package index

import (
	"testing"

	"mvn-search/internal/maven"
)

func sample() []maven.Artifact {
	return []maven.Artifact{
		{GroupID: "com.google.guava", ArtifactID: "guava", Version: "33.5.0-jre"},
		{GroupID: "com.google.guava", ArtifactID: "guava", Version: "33.4.0-jre"},
		{GroupID: "com.google.guava", ArtifactID: "failureaccess", Version: "1.0.3"},
		{GroupID: "org.junit.jupiter", ArtifactID: "junit-jupiter", Version: "5.10.0"},
		{GroupID: "commons-io", ArtifactID: "commons-io", Version: "2.16.1"},
	}
}

func TestSearchSingleKeyword(t *testing.T) {
	// "guava" matches artifactId guava and groupId com.google.guava
	got := Search(sample(), "guava")
	if len(got) != 3 {
		t.Fatalf("want 3 hits, got %d: %+v", len(got), got)
	}
	if got[0].ArtifactID != "guava" {
		t.Fatalf("want guava first, got %s", got[0].ArtifactID)
	}
}

func TestSearchIntersection(t *testing.T) {
	// both keywords must hit; "google guava" keeps only com.google.guava artifacts
	got := Search(sample(), "google guava")
	if len(got) != 3 {
		t.Fatalf("want 3 hits, got %d: %+v", len(got), got)
	}
	for _, a := range got {
		if a.GroupID != "com.google.guava" {
			t.Fatalf("unexpected hit %s", a.GAV())
		}
	}
	// intersection must drop junit / commons-io
	got2 := Search(sample(), "guava junit")
	if len(got2) != 0 {
		t.Fatalf("want 0 intersection hits, got %d: %+v", len(got2), got2)
	}
}

func TestSearchNoMatch(t *testing.T) {
	got := Search(sample(), "zzz-not-here")
	if len(got) != 0 {
		t.Fatalf("want 0 hits, got %d", len(got))
	}
}

func TestSearchCaseInsensitive(t *testing.T) {
	got := Search(sample(), "Junit")
	if len(got) != 1 || got[0].ArtifactID != "junit-jupiter" {
		t.Fatalf("unexpected hits: %+v", got)
	}
}

func TestSearchEmptyReturnsSorted(t *testing.T) {
	got := Search(sample(), "  ")
	if len(got) != 5 {
		t.Fatalf("want all 5, got %d", len(got))
	}
}
