package upgradetest

import (
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

// UpgradeSuite groups the operator-index upgrade tests.
// BeforeTest resets leftover operator state before each test so that local
// re-runs on the same cluster are safe without manual cleanup.
type UpgradeSuite struct {
	suite.Suite
}

func (s *UpgradeSuite) BeforeTest(_, _ string) {
	s.Require().NoError(ResetOperator(), "pre-test reset")
}

func TestUpgradeSuite(t *testing.T) {
	suite.Run(t, new(UpgradeSuite))
}

func requireEnv(t *testing.T, name string) string {
	t.Helper()
	val := os.Getenv(name)
	if val == "" {
		t.Fatalf("%s env var must be set", name)
	}
	return val
}
