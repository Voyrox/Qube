package testutil

import (
	"testing"

	"github.com/Voyrox/Qube/internal/config"
)

func TestOverridePaths(t *testing.T) {
	origBase := config.QubeContainersBase
	cleanup := OverridePaths(t)
	if config.QubeContainersBase == origBase {
		t.Fatalf("expected override")
	}
	cleanup()
	if config.QubeContainersBase != origBase {
		t.Fatalf("expected restore")
	}
}
