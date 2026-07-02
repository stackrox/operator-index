#!/usr/bin/env bash
# Test 1 — Upgrade from oldest supported to provided version
#
# 1. Installs ACS Operator (oldest_supported_version from bundles.yaml) from official redhat-operators
# 2. Upgrades to the provided OPERATOR_INDEX_IMAGE on the ACS_VERSION channel
# 3. Verifies CSV reaches Succeeded after each step
#
# Required env vars:
#   OPERATOR_INDEX_IMAGE   custom operator index image
#   ACS_VERSION            ACS minor version to test, e.g. "4.10"

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "${SCRIPT_DIR}/lib.sh"

OPERATOR_INDEX_IMAGE="${OPERATOR_INDEX_IMAGE:-${MY_INDEX_IMAGE:-}}"
ACS_VERSION="${ACS_VERSION:-}"

[[ -n "$OPERATOR_INDEX_IMAGE" ]] || fail "OPERATOR_INDEX_IMAGE is required"
[[ -n "$ACS_VERSION" ]]          || fail "ACS_VERSION is required (e.g. 4.10)"

parse_acs_version "$ACS_VERSION"
CHANNEL="rhacs-${ACS_MAJOR}.${ACS_MINOR}"

BUNDLES_YAML="${SCRIPT_DIR}/../bundles.yaml"
[[ -f "$BUNDLES_YAML" ]] || fail "bundles.yaml not found at ${BUNDLES_YAML}"
OLDEST_VERSION=$(awk '/^oldest_supported_version:/{print $2; exit}' "$BUNDLES_YAML")
[[ -n "$OLDEST_VERSION" ]] || fail "oldest_supported_version not found in bundles.yaml"
OLDEST_MAJOR=$(echo "$OLDEST_VERSION" | cut -d. -f1)
OLDEST_MINOR=$(echo "$OLDEST_VERSION" | cut -d. -f2)

echo "======================================================================"
echo "  TEST 1: Upgrade oldest-supported → provided"
echo "  Image:   ${OPERATOR_INDEX_IMAGE}"
echo "  Version: ${ACS_MAJOR}.${ACS_MINOR} | Channel: ${CHANNEL}"
echo "  Oldest:  ${OLDEST_MAJOR}.${OLDEST_MINOR} (from bundles.yaml)"
echo "======================================================================"

step "Step 1: Install ACS Operator ${OLDEST_MAJOR}.${OLDEST_MINOR} from redhat-operators"
install_from_official "$OLDEST_MAJOR" "$OLDEST_MINOR"
wait_for_csv "$TARGET_CSV" 300
log_csv

step "Step 2: Upgrade to ${CHANNEL} via custom index"
upgrade_via_custom "$OPERATOR_INDEX_IMAGE" "$CHANNEL"
wait_for_csv "$TARGET_CSV" 1200
log_csv

echo
echo "======================================================================"
echo "  ✅ TEST 1 PASSED"
echo "======================================================================"
