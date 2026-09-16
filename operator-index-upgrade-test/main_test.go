package upgradetest

import (
	"fmt"
	"os"
	"testing"
)

// TestMain resets any leftover operator state before running the suite.
// This makes local re-runs on the same cluster safe without manual cleanup.
func TestMain(m *testing.M) {
	if err := ResetOperator(); err != nil {
		fmt.Fprintf(os.Stderr, "pre-test reset failed: %v\n", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func requireEnv(t *testing.T, name string) string {
	t.Helper()
	val := os.Getenv(name)
	if val == "" {
		t.Fatalf("%s env var must be set", name)
	}
	return val
}
