package tracking

import (
	"os"
	"testing"

	"github.com/Voyrox/Qube/internal/config"
	"github.com/Voyrox/Qube/internal/testutil"
)

func TestTrackAndGetAll(t *testing.T) {
	cleanup := testutil.OverridePaths(t)
	defer cleanup()

	if err := TrackContainerNamed("c1", 123, "/dir", []string{"cmd"}, "img", "8080", true, nil, nil); err != nil {
		t.Fatalf("TrackContainerNamed error: %v", err)
	}

	entries := GetAllTrackedEntries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Name != "c1" || e.PID != 123 || e.Dir != "/dir" || e.Image != "img" || e.Ports != "8080" || !e.Isolated {
		t.Fatalf("entry mismatch: %#v", e)
	}
}

func TestUpdateAndRemove(t *testing.T) {
	cleanup := testutil.OverridePaths(t)
	defer cleanup()

	if err := TrackContainerNamed("c2", 1, "/dir", []string{"cmd"}, "img", "", false, nil, nil); err != nil {
		t.Fatalf("track: %v", err)
	}

	if err := UpdateContainerPID("c2", 2, "/dir2", []string{"bash"}, "img2", "", true, nil, nil); err != nil {
		t.Fatalf("update: %v", err)
	}

	entries := GetAllTrackedEntries()
	if len(entries) != 1 || entries[0].PID != 2 || entries[0].Dir != "/dir2" || entries[0].Image != "img2" || !entries[0].Isolated {
		t.Fatalf("update mismatch: %#v", entries)
	}

	if err := RemoveContainerFromTracking(entries[0].PID); err != nil {
		t.Fatalf("remove by pid: %v", err)
	}
	if len(GetAllTrackedEntries()) != 0 {
		t.Fatalf("expected empty after remove by pid")
	}

	// Re-add and remove by name
	if err := TrackContainerNamed("c3", 10, "/dir", []string{"cmd"}, "img", "", false, nil, nil); err != nil {
		t.Fatalf("track c3: %v", err)
	}
	if err := RemoveContainerFromTrackingByName("c3"); err != nil {
		t.Fatalf("remove by name: %v", err)
	}
	if len(GetAllTrackedEntries()) != 0 {
		t.Fatalf("expected empty after remove by name")
	}
}

func TestGetProcessUptimeInvalid(t *testing.T) {
	if _, err := GetProcessUptime(-1); err == nil {
		t.Fatalf("expected error for invalid pid")
	}
}

func TestContainerListFileEnsureNewline(t *testing.T) {
	cleanup := testutil.OverridePaths(t)
	defer cleanup()

	// Manually write a file without trailing newline to ensure TrackContainerNamed appends correctly.
	if err := os.WriteFile(config.ContainerListFile, []byte("existing|1|/|cmd|0|img|ports|true"), 0644); err != nil {
		t.Fatalf("write seed: %v", err)
	}

	if err := TrackContainerNamed("c4", 4, "/d", []string{"c"}, "i", "p", false, nil, nil); err != nil {
		t.Fatalf("track c4: %v", err)
	}

	data, err := os.ReadFile(config.ContainerListFile)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data[len(data)-1]) != "\n" {
		t.Fatalf("expected trailing newline, got: %q", data[len(data)-1])
	}
}
