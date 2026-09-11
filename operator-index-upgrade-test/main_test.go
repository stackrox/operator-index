package upgradetest

import (
	"os"
	"testing"
)

// TestMain resets any leftover operator state before running the suite.
// This makes local re-runs on the same cluster safe without manual cleanup.
func TestMain(m *testing.M) {
	_ = ResetOperator()
	os.Exit(m.Run())
}
