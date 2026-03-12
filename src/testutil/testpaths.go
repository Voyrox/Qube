package testutil

import (
	"testing"

	"github.com/Voyrox/Qube/src/config"
)

func OverridePaths(t *testing.T) func() {
	t.Helper()
	tmp := t.TempDir()
	return config.SetPathsForTests(tmp)
}
