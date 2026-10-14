package maven

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanStandardLayout(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "com/google/guava/guava/33.5.0-jre/guava-33.5.0-jre.pom"), "<project/>")
	mustWrite(t, filepath.Join(root, "com/google/guava/guava/33.5.0-jre/guava-33.5.0-jre.jar"), "jar")
	mustWrite(t, filepath.Join(root, "com/google/guava/guava/33.5.0-jre/guava-33.5.0-jre-sources.jar"), "src")
	mustWrite(t, filepath.Join(root, "com/google/guava/guava/33.4.0-jre/guava-33.4.0-jre.pom"), "<project/>")
	mustWrite(t, filepath.Join(root, "net/example/tool/1.0.9/tool-1.0.9.jar"), "jar")
	mustWrite(t, filepath.Join(root, "net/example/tool/1.0.9/tool-1.0.9-sources.jar"), "src")

	got, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 artifacts, got %d: %+v", len(got), got)
	}
	// jar preferred over pom for same GAV
	var guava *Artifact
	for i := range got {
		if got[i].GroupID == "com.google.guava" && got[i].Version == "33.5.0-jre" {
			guava = &got[i]
		}
	}
	if guava == nil {
		t.Fatalf("missing guava 33.5.0-jre: %+v", got)
	}
	if guava.Packaging != "jar" {
		t.Fatalf("want packaging jar, got %s", guava.Packaging)
	}
	if guava.ArtifactID != "guava" || guava.GroupID != "com.google.guava" {
		t.Fatalf("bad coords: %+v", guava)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
