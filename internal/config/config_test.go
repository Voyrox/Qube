package config

import "testing"

func TestSetPathsForTests(t *testing.T) {
	origBase := QubeContainersBase
	origCgroup := CgroupRoot
	origTrack := TrackingDir
	origFile := ContainerListFile

	cleanup := SetPathsForTests(t.TempDir())
	if QubeContainersBase == origBase || TrackingDir == origTrack {
		t.Fatalf("paths not overridden")
	}
	cleanup()
	if QubeContainersBase != origBase || CgroupRoot != origCgroup || TrackingDir != origTrack || ContainerListFile != origFile {
		t.Fatalf("paths not restored")
	}
}
