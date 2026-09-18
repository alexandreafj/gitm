package git_test

import (
	"os"
	"testing"
)

// TestMain isolates every git invocation in this package from the developer's
// own git configuration. A global core.hooksPath (lefthook, commit-msg
// linters) would otherwise run against the throwaway repos and fail commits.
func TestMain(m *testing.M) {
	os.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	os.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	os.Exit(m.Run())
}
