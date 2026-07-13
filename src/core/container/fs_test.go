package container

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveMountDestinationRejectsTraversal(t *testing.T) {
	rootfs := filepath.Join(t.TempDir(), "rootfs")

	if _, err := resolveMountDestination(rootfs, "../etc"); err == nil {
		t.Fatal("expected traversal path to be rejected")
	}

	if _, err := resolveMountDestination(rootfs, "workspace"); err == nil {
		t.Fatal("expected relative path to be rejected")
	}
}

func TestResolveMountDestinationAcceptsAbsoluteContainerPath(t *testing.T) {
	rootfs := filepath.Join(t.TempDir(), "rootfs")

	dest, err := resolveMountDestination(rootfs, "/workspace/data")
	if err != nil {
		t.Fatalf("resolveMountDestination() error = %v", err)
	}

	if !strings.HasPrefix(dest, rootfs) {
		t.Fatalf("expected destination %q to stay under rootfs %q", dest, rootfs)
	}
}
