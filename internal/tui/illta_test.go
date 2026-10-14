package tui

import (
	"strings"
	"testing"

	"mvn-search/internal/maven"
)

func TestIlltaLayout(t *testing.T) {
	results := arts(
		maven.Artifact{GroupID: "com.illtamer.infinite.bot", ArtifactID: "minecraft", Version: "4.2.0-SNAPSHOT"},
		maven.Artifact{GroupID: "com.illtamer.infinite.bot", ArtifactID: "minecraft", Version: "4.1.0-SNAPSHOT"},
		maven.Artifact{GroupID: "com.illtamer.infinite.bot", ArtifactID: "minecraft", Version: "4.0.4-SNAPSHOT"},
		maven.Artifact{GroupID: "com.illtamer.lib", ArtifactID: "easy-plugin", Version: "1.0.2-SNAPSHOT"},
		maven.Artifact{GroupID: "com.illtamer.lib", ArtifactID: "easy-plugin", Version: "1.0.1-SNAPSHOT"},
		maven.Artifact{GroupID: "com.illtamer.lib", ArtifactID: "easy-plugin", Version: "1.0.0-SNAPSHOT"},
		maven.Artifact{GroupID: "com.illtamer.perpetua.sdk", ArtifactID: "perpetua-sdk", Version: "0.2.3-SNAPSHOT"},
		maven.Artifact{GroupID: "com.illtamer.perpetua.sdk", ArtifactID: "perpetua-sdk", Version: "0.2.2-SNAPSHOT"},
		maven.Artifact{GroupID: "com.illtamer.plugin", ArtifactID: "custom-vault", Version: "1.0-SNAPSHOT"},
		maven.Artifact{GroupID: "com.illtamer.plugin", ArtifactID: "functions", Version: "1.0-SNAPSHOT"},
		maven.Artifact{GroupID: "com.illtamer.plugin.acquisitionstore", ArtifactID: "acquisition-store", Version: "1.0-SNAPSHOT"},
		maven.Artifact{GroupID: "com.illtamer.plugin.activityreward", ArtifactID: "activity-reward", Version: "1.0-SNAPSHOT"},
		maven.Artifact{GroupID: "com.illtamer.plugin.infinitetime", ArtifactID: "infinite-time", Version: "1.0-SNAPSHOT"},
		maven.Artifact{GroupID: "com.illtamer.plugin.magicgemtiny", ArtifactID: "magic-gem-tiny", Version: "1.0-SNAPSHOT"},
	)
	got := buildRowsWithQuery(results, "com.illta", nil)
	// ensure nested groups hang under plugin, not at root
	for _, r := range got {
		if r.depth == 0 && strings.Contains(r.text, "acquisition") {
			t.Fatalf("bad root: %+v", r)
		}
	}
	pluginIdx := -1
	for i, r := range got {
		if r.kind == rowGroup && r.text == "plugin" {
			pluginIdx = i
			break
		}
	}
	if pluginIdx < 0 {
		t.Fatalf("missing plugin node: %+v", got)
	}
	pluginKey := got[pluginIdx].key
	want := []string{"custom-vault", "functions", "acquisitionstore:acquisition-store", "activityreward:activity-reward", "infinitetime:infinite-time", "magicgemtiny:magic-gem-tiny"}
	var kids []string
	for _, r := range got {
		if r.parent == pluginKey {
			kids = append(kids, r.text)
		}
	}
	if strings.Join(kids, "|") != strings.Join(want, "|") {
		t.Fatalf("plugin children order/content:\n got %v\nwant %v", kids, want)
	}
}
