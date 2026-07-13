package tracking

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Voyrox/Qube/src/config"
)

func TestTrackContainerNamedRoundTripPersistsVolumesAndEnvVars(t *testing.T) {
	tempDir := t.TempDir()
	cleanup := config.SetPathsForTests(tempDir)
	defer cleanup()

	volumes := [][2]string{{"/host/data", "/container/data"}, {"/host/cache", "/container/cache"}}
	envVars := []string{"FOO=bar", "BAZ=qux"}

	if err := TrackContainerNamed("demo", 1234, "/work", []string{"sleep", "60"}, "demo-image", "8080:80", true, volumes, envVars); err != nil {
		t.Fatalf("TrackContainerNamed() error = %v", err)
	}

	entries := GetAllTrackedEntries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if !reflect.DeepEqual(entry.Volumes, volumes) {
		t.Fatalf("expected volumes %v, got %v", volumes, entry.Volumes)
	}
	if !reflect.DeepEqual(entry.EnvVars, envVars) {
		t.Fatalf("expected env vars %v, got %v", envVars, entry.EnvVars)
	}

	content, err := os.ReadFile(config.ContainerListFile)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if filepath.Ext(config.ContainerListFile) != ".txt" || len(content) == 0 {
		t.Fatalf("expected tracking file to be written")
	}
}
