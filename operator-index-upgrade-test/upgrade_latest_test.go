package upgradetest

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestUpgradeLatest tests the install + optional upgrade to latest GA path:
//  1. Installs ACS Operator from OPERATOR_INDEX_IMAGE on the ACS_VERSION channel.
//  2. If ACS_VERSION minor < latest GA minor for the same major: upgrades to latest GA via redhat-operators.
//  3. Verifies each CSV reaches Succeeded.
func TestUpgradeLatest(t *testing.T) {
	operatorIndexImage := requireEnv(t, "OPERATOR_INDEX_IMAGE")
	acsVersion := requireEnv(t, "ACS_VERSION")

	major, minor, err := ParseACSVersion(acsVersion)
	require.NoError(t, err, "parse ACS_VERSION")
	channel := fmt.Sprintf("rhacs-%d.%d", major, minor)

	t.Cleanup(func() { _ = ResetOperator() })

	t.Logf("Image:   %s", operatorIndexImage)
	t.Logf("Version: %d.%d | Channel: %s", major, minor, channel)

	// Query latest GA minor BEFORE disabling default sources (install_from_custom disables them).
	// Use || to degrade gracefully: a catalog timeout just skips the upgrade check.
	var latestMinor int
	if err := WaitForCatalog("redhat-operators", "openshift-marketplace", 3*time.Minute); err != nil {
		t.Logf("Warning: could not wait for redhat-operators: %v — upgrade check will be skipped", err)
	} else if lm, err := GetLatestOfficialMinor(major, 3*time.Minute); err != nil {
		t.Logf("Warning: could not determine latest GA minor for %d.x: %v — upgrade check will be skipped", major, err)
	} else {
		latestMinor = lm
		t.Logf("Latest GA in redhat-operators: %d.%d", major, latestMinor)
	}

	t.Logf("Step 1: Install ACS Operator %s from custom index", channel)
	targetCSV, err := InstallFromCustom(operatorIndexImage, channel)
	require.NoError(t, err)
	require.NoError(t, WaitForCSV(targetCSV, 10*time.Minute))

	// Upgrade only when the tested major matches and our minor is behind latest.
	// Comparing major prevents a spurious upgrade when testing a pre-release (e.g. 5.0 vs 4.12).
	if latestMinor > 0 && minor < latestMinor {
		t.Logf("Step 2: Upgrade %d.%d → %d.%d (latest GA)", major, minor, major, latestMinor)
		targetCSV, err = UpgradeToLatestOfficial(major)
		require.NoError(t, err)
		require.NoError(t, WaitForCSV(targetCSV, 10*time.Minute))
	} else if latestMinor > 0 {
		t.Logf("%d.%d is already at or ahead of the latest GA minor (%d) — no upgrade needed",
			major, minor, latestMinor)
	}
}
