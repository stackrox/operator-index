package upgradetest

import (
	"fmt"
	"time"

	"github.com/stretchr/testify/require"
)

// TestUpgradeLatest tests the install + optional upgrade to latest GA path:
//  1. Installs ACS Operator from OPERATOR_INDEX_IMAGE on the VERSION_STREAM channel and verifies the CSV reaches Succeeded.
//  2. If VERSION_STREAM < latest GA minor: upgrades to latest GA via redhat-operators and verifies the CSV reaches Succeeded.
func (s *UpgradeSuite) TestUpgradeLatest() {
	t := s.T()
	operatorIndexImage := requireEnv(t, "OPERATOR_INDEX_IMAGE")
	versionStream := requireEnv(t, "VERSION_STREAM")

	stream, err := ParseVersionStream(versionStream)
	require.NoError(t, err, "parse VERSION_STREAM")
	channel := fmt.Sprintf("rhacs-%d.%d", stream.Major(), stream.Minor())

	t.Cleanup(func() { require.NoError(t, ResetOperator()) })

	t.Logf("Image:   %s", operatorIndexImage)
	t.Logf("Version: %d.%d | Channel: %s", stream.Major(), stream.Minor(), channel)

	// Query latest GA stream BEFORE disabling default sources (install_from_custom disables them).
	require.NoError(t, WaitForCatalog("redhat-operators", "openshift-marketplace", 3*time.Minute))
	latestStream, err := GetLatestOfficialStream(3 * time.Minute)
	require.NoError(t, err)
	t.Logf("Latest GA in redhat-operators: %d.%d", latestStream.Major(), latestStream.Minor())

	t.Logf("Step 1: Install ACS Operator %s from custom index", channel)
	targetCSV, err := InstallFromCustom(operatorIndexImage, channel)
	require.NoError(t, err)
	require.NoError(t, WaitForCSV(targetCSV, 10*time.Minute))

	if stream.LessThan(latestStream) {
		t.Logf("Step 2: Upgrade %d.%d → %d.%d (latest GA)", stream.Major(), stream.Minor(), latestStream.Major(), latestStream.Minor())
		targetCSV, err = UpgradeToLatestOfficial(latestStream)
		require.NoError(t, err)
		require.NoError(t, WaitForCSV(targetCSV, 10*time.Minute))
	} else {
		t.Logf("%d.%d is already at or ahead of the latest GA %d.%d — no upgrade needed",
			stream.Major(), stream.Minor(), latestStream.Major(), latestStream.Minor())
	}
}
