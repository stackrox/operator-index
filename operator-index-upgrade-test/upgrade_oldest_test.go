package upgradetest

import (
	"fmt"
	"time"

	"github.com/stretchr/testify/require"
)

// TestUpgradeOldest tests the upgrade path from the oldest supported version to the provided index:
//  1. Installs ACS Operator (oldest_supported_version from bundles.yaml) from official redhat-operators and verifies the CSV reaches Succeeded.
//  2. Upgrades to OPERATOR_INDEX_IMAGE on the VERSION_STREAM channel and verifies the CSV reaches Succeeded.
func (s *UpgradeSuite) TestUpgradeOldest() {
	t := s.T()
	operatorIndexImage := requireEnv(t, "OPERATOR_INDEX_IMAGE")
	versionStream := requireEnv(t, "VERSION_STREAM")

	stream, err := ParseVersionStream(versionStream)
	require.NoError(t, err, "parse VERSION_STREAM")
	channel := fmt.Sprintf("rhacs-%d.%d", stream.Major(), stream.Minor())

	oldest, err := ReadOldestSupportedVersion()
	require.NoError(t, err, "read oldest_supported_version from bundles.yaml")

	t.Cleanup(func() { require.NoError(t, ResetOperator()) })

	t.Logf("Image:   %s", operatorIndexImage)
	t.Logf("Version: %d.%d | Channel: %s", stream.Major(), stream.Minor(), channel)
	t.Logf("Oldest:  %d.%d (from bundles.yaml)", oldest.Major(), oldest.Minor())

	t.Logf("Step 1: Install ACS Operator %d.%d from redhat-operators", oldest.Major(), oldest.Minor())
	targetCSV, err := InstallFromOfficial(oldest)
	require.NoError(t, err)
	require.NoError(t, WaitForCSV(targetCSV, 10*time.Minute))

	t.Logf("Step 2: Upgrade to %s via custom index", channel)
	targetCSV, err = UpgradeViaCustom(operatorIndexImage, channel)
	require.NoError(t, err)
	require.NoError(t, WaitForCSV(targetCSV, 30*time.Minute))
}
