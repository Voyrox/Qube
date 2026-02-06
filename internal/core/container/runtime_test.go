package container

import (
	"os"
	"testing"

	"github.com/Voyrox/Qube/internal/testutil"
)

func TestRunContainerMissingCmd(t *testing.T) {
	if err := RunContainer("", t.TempDir(), nil, false, "img", "", false, nil, nil); err == nil {
		t.Fatalf("expected error for missing command")
	}
}

func TestBuildContainerSkipsExisting(t *testing.T) {
	cleanup := testutil.OverridePaths(t)
	defer cleanup()

	cid := "exists"
	root := GetRootfs(cid)
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}

	if _, err := BuildContainer(cid, t.TempDir(), "img"); err != nil {
		t.Fatalf("BuildContainer with existing rootfs should succeed: %v", err)
	}
}
