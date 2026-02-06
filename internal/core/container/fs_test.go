package container

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Voyrox/Qube/internal/testutil"
)

func TestCopyFile(t *testing.T) {
	cleanup := testutil.OverridePaths(t)
	defer cleanup()

	srcDir := t.TempDir()
	dstDir := t.TempDir()

	src := filepath.Join(srcDir, "a.txt")
	dst := filepath.Join(dstDir, "b.txt")
	if err := os.WriteFile(src, []byte("hello"), 0644); err != nil {
		t.Fatalf("write src: %v", err)
	}

	if err := CopyFile(src, dst); err != nil {
		t.Fatalf("CopyFile error: %v", err)
	}

	out, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(out) != "hello" {
		t.Fatalf("content mismatch: %q", string(out))
	}
}

func TestCopyDir(t *testing.T) {
	cleanup := testutil.OverridePaths(t)
	defer cleanup()

	src := filepath.Join(t.TempDir(), "src")
	dst := filepath.Join(t.TempDir(), "dst")
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "file.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if err := CopyDir(src, dst); err != nil {
		t.Fatalf("CopyDir error: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(dst, "sub", "file.txt"))
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(out) != "data" {
		t.Fatalf("content mismatch: %q", string(out))
	}
}

func TestPrepareRootfsDir(t *testing.T) {
	cleanup := testutil.OverridePaths(t)
	defer cleanup()

	cid := "abc"
	if err := PrepareRootfsDir(cid); err != nil {
		t.Fatalf("PrepareRootfsDir error: %v", err)
	}
	root := GetRootfs(cid)
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		t.Fatalf("rootfs missing: err=%v, dir=%v", err, st != nil && st.IsDir())
	}
}

func TestMountVolumeValidation(t *testing.T) {
	cleanup := testutil.OverridePaths(t)
	defer cleanup()

	cid := "voltest"
	PrepareRootfsDir(cid)

	err := MountVolume(cid, filepath.Join(t.TempDir(), "nope"), "/data")
	if err == nil {
		t.Fatalf("expected error for missing host path")
	}
}

func TestCopyDirectoryIntoHomeFallback(t *testing.T) {
	cleanup := testutil.OverridePaths(t)
	defer cleanup()

	cid := "copydir"
	workDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workDir, "file.txt"), []byte("x"), 0644); err != nil {
		t.Fatalf("write work file: %v", err)
	}

	if err := PrepareRootfsDir(cid); err != nil {
		t.Fatalf("prepare rootfs: %v", err)
	}

	if err := CopyDirectoryIntoHome(cid, workDir); err != nil {
		t.Fatalf("CopyDirectoryIntoHome error: %v", err)
	}

	copied := filepath.Join(GetRootfs(cid), "workspace", "file.txt")
	if _, err := os.Stat(copied); err != nil {
		t.Fatalf("copied file missing: %v", err)
	}
}
