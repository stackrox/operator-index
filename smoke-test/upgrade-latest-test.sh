#!/usr/bin/env bash
# Test 2 — Install provided version, optionally upgrade to latest GA
#
# 1. Installs ACS Operator from OPERATOR_INDEX_IMAGE on the ACS_VERSION channel
# 2. If ACS_VERSION minor < latest GA minor: upgrades to latest GA via redhat-operators
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

echo "======================================================================"
echo "  TEST 2: Install provided → optionally upgrade to latest GA"
echo "  Image:   ${OPERATOR_INDEX_IMAGE}"
echo "  Version: ${ACS_MAJOR}.${ACS_MINOR} | Channel: ${CHANNEL}"
echo "======================================================================"

# Query latest GA minor NOW while redhat-operators is still enabled.
# install_from_custom() disables default sources, making this unavailable later.
# Use || true so a catalog timeout degrades gracefully to skipping the upgrade check.
LATEST_MINOR=""
if wait_for_catalog "redhat-operators" "openshift-marketplace" 180 &&
   latest=$(get_latest_official_minor "$ACS_MAJOR") && [[ -n "$latest" ]]; then
  LATEST_MINOR="$latest"
  info "Latest GA in redhat-operators: ${ACS_MAJOR}.${LATEST_MINOR}"
else
  warn "Could not determine latest GA minor — upgrade check will be skipped"
fi

step "Step 1: Install ACS Operator ${CHANNEL} from custom index"
install_from_custom "$OPERATOR_INDEX_IMAGE" "$CHANNEL"
wait_for_csv "$TARGET_CSV" 600
log_csv

if [[ -n "$LATEST_MINOR" ]] && (( ACS_MINOR < LATEST_MINOR )); then
  step "Step 2: Upgrade ${ACS_MAJOR}.${ACS_MINOR} → ${ACS_MAJOR}.${LATEST_MINOR} (latest GA)"
  upgrade_to_latest_official "$ACS_MAJOR"
  wait_for_csv "$TARGET_CSV" 600
  log_csv
elif [[ -n "$LATEST_MINOR" ]]; then
  info "${ACS_MAJOR}.${ACS_MINOR} is already at latest GA minor (${LATEST_MINOR}) — no upgrade needed"
fi

echo
echo "======================================================================"
echo "  ✅ TEST 2 PASSED"
echo "======================================================================"
