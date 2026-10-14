package maven

import "testing"

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0", "1.0", 0},
		{"1.0", "1.0.1", -1},
		{"1.0.1", "1.0", 1},
		{"33.10.0", "33.9.0", 1},
		{"33.5.0-jre", "33.5.0-jre", 0},
		{"33.5.0-jre", "33.5.0-android", 1},
		{"1.0.0", "1.0.0-SNAPSHOT", 1},
		{"2.0", "10.0", -1},
		{"1.25.5", "1.25.3", 1},
		{"33.5.0-jre", "33.5.0", -1},
	}
	for _, c := range cases {
		got := CompareVersions(c.a, c.b)
		if got != c.want {
			t.Fatalf("CompareVersions(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestFormatMavenAndGradle(t *testing.T) {
	a := Artifact{GroupID: "com.google.guava", ArtifactID: "guava", Version: "33.5.0-jre"}
	wantMvn := "<dependency>\n    <groupId>com.google.guava</groupId>\n    <artifactId>guava</artifactId>\n    <version>33.5.0-jre</version>\n</dependency>"
	if got := a.FormatMaven(); got != wantMvn {
		t.Fatalf("FormatMaven:\n%s\nwant:\n%s", got, wantMvn)
	}
	wantGradle := "implementation 'com.google.guava:guava:33.5.0-jre'"
	if got := a.FormatGradle(); got != wantGradle {
		t.Fatalf("FormatGradle=%q want %q", got, wantGradle)
	}
}
