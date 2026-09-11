package upgradetest

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func requireEnv(t *testing.T, name string) string {
	t.Helper()
	val := os.Getenv(name)
	require.NotEmptyf(t, val, "%s env var must be set", name)
	return val
}

// TestUpgradeOldest tests the upgrade path from the oldest supported version to the provided index:
//  1. Installs ACS Operator (oldest_supported_version from bundles.yaml) from official redhat-operators.
//  2. Upgrades to OPERATOR_INDEX_IMAGE on the ACS_VERSION channel.
//  3. Verifies each CSV reaches Succeeded.
func TestUpgradeOldest(t *testing.T) {
	operatorIndexImage := requireEnv(t, "OPERATOR_INDEX_IMAGE")
	acsVersion := requireEnv(t, "ACS_VERSION")

	major, minor, err := ParseACSVersion(acsVersion)
	require.NoError(t, err, "parse ACS_VERSION")
	channel := fmt.Sprintf("rhacs-%d.%d", major, minor)

	oldestMajor, oldestMinor, err := ReadOldestSupportedVersion()
	require.NoError(t, err, "read oldest_supported_version from bundles.yaml")

	t.Cleanup(func() { _ = ResetOperator() })

	t.Logf("Image:   %s", operatorIndexImage)
	t.Logf("Version: %d.%d | Channel: %s", major, minor, channel)
	t.Logf("Oldest:  %d.%d (from bundles.yaml)", oldestMajor, oldestMinor)

	t.Logf("Step 1: Install ACS Operator %d.%d from redhat-operators", oldestMajor, oldestMinor)
	targetCSV, err := InstallFromOfficial(oldestMajor, oldestMinor)
	require.NoError(t, err)
	require.NoError(t, WaitForCSV(targetCSV, 10*time.Minute))

	t.Logf("Step 2: Upgrade to %s via custom index", channel)
	targetCSV, err = UpgradeViaCustom(operatorIndexImage, channel)
	require.NoError(t, err)
	require.NoError(t, WaitForCSV(targetCSV, 30*time.Minute))
}
