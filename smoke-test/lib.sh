#!/usr/bin/env bash
# Shared functions for ACS operator smoke tests.

set -euo pipefail

# Logging helpers
step() { echo; echo "══════════════════════════════════════════════════════"; echo "  $*"; echo "══════════════════════════════════════════════════════"; }
info() { echo "  $*"; }
ok()   { echo "  ✅ $*"; }
warn() { echo "  ⚠️  $*"; }
fail() { echo "  ❌ $*" >&2; exit 1; }

# Splits "4.10" → ACS_MAJOR=4, ACS_MINOR=10
parse_acs_version() {
  local ver="$1"
  ACS_MAJOR=$(echo "$ver" | cut -d. -f1)
  ACS_MINOR=$(echo "$ver" | cut -d. -f2)
  [[ "$ACS_MAJOR" =~ ^[0-9]+$ && "$ACS_MINOR" =~ ^[0-9]+$ ]] \
    || fail "ACS_VERSION must be MAJOR.MINOR (e.g. 4.10), got: ${ver}"
}

# Prints the highest minor version in rhacs-MAJOR.N channels from the official catalog.
get_latest_official_minor() {
  local major="${1:-4}"
  oc get packagemanifest -n openshift-marketplace \
    -l catalog=redhat-operators \
    -o jsonpath='{range .items[?(@.metadata.name=="rhacs-operator")].status.channels[*]}{.name}{"\n"}{end}' \
    | grep -oE "rhacs-${major}\.[0-9]+" \
    | grep -oE '[0-9]+$' \
    | sort -n \
    | tail -1 \
    || true
}

disable_default_sources() {
  info "Disabling default OperatorHub sources..."
  oc patch OperatorHub cluster --type json \
    -p '[{"op":"add","path":"/spec/disableAllDefaultSources","value":true}]'
  ok "Default sources disabled"
}

enable_default_sources() {
  info "Enabling default OperatorHub sources..."
  oc patch OperatorHub cluster --type json \
    -p '[{"op":"add","path":"/spec/disableAllDefaultSources","value":false}]'
  ok "Default sources enabled"
}

apply_custom_catalog() {
  local index_image="$1"
  info "Applying custom CatalogSource (image: ${index_image})..."
  oc apply -f - <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: my-operator-catalog
  namespace: openshift-marketplace
spec:
  sourceType: grpc
  image: ${index_image}
  displayName: My Operator Catalog
  publisher: grpc
EOF
  ok "Custom CatalogSource applied"
}

wait_for_catalog() {
  local name="$1"
  local ns="${2:-openshift-marketplace}"
  local timeout="${3:-180}"
  local deadline
  deadline=$(( $(date +%s) + timeout ))
  info "Waiting for CatalogSource/${name} to be READY (timeout: ${timeout}s)..."
  while (( $(date +%s) < deadline )); do
    local state
    state=$(oc get catalogsource "$name" -n "$ns" \
      -o jsonpath='{.status.connectionState.lastObservedState}' 2>/dev/null || true)
    if [[ "$state" == "READY" ]]; then
      ok "CatalogSource/${name} is READY"
      return 0
    fi
    info "  state=${state:-pending}, retrying in 10s..."
    sleep 10
  done
  fail "CatalogSource/${name} not READY within ${timeout}s"
}

apply_subscription() {
  local channel="$1" source="$2" source_ns="${3:-openshift-marketplace}"
  info "Applying Subscription (channel=${channel}, source=${source})..."
  oc apply -f - <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: rhacs-operator
  namespace: openshift-operators
spec:
  channel: ${channel}
  installPlanApproval: Automatic
  name: rhacs-operator
  source: ${source}
  sourceNamespace: ${source_ns}
EOF
  ok "Subscription applied"
}

# Sets TARGET_CSV to the currentCSV for a given channel from the specified catalog label.
# Retries for up to timeout (default 60s) because packagemanifests can lag behind catalog READY state.
_resolve_target_csv() {
  local catalog_label="$1" channel="$2" timeout="${3:-60}"
  local deadline
  deadline=$(( $(date +%s) + timeout ))
  info "Resolving currentCSV for channel ${channel} from ${catalog_label}..."
  while (( $(date +%s) < deadline )); do
    TARGET_CSV=$(oc get packagemanifest -n openshift-marketplace \
      -l "catalog=${catalog_label}" \
      -o jsonpath="{range .items[?(@.metadata.name==\"rhacs-operator\")].status.channels[?(@.name==\"${channel}\")]}{.currentCSV}{end}" \
      2>/dev/null || true)
    if [[ -n "$TARGET_CSV" ]]; then
      info "Target CSV: ${TARGET_CSV}"
      return 0
    fi
    info "  packagemanifest not ready yet, retrying in 10s..."
    sleep 10
  done
  fail "Could not resolve currentCSV for channel ${channel} from ${catalog_label} after ${timeout}s"
}

# Install from official redhat-operators at channel rhacs-MAJOR.MINOR.
# Sets TARGET_CSV to the channel's currentCSV.
install_from_official() {
  local major="$1" minor="$2"
  local channel="rhacs-${major}.${minor}"
  info "Installing ACS Operator from redhat-operators, channel ${channel}..."
  wait_for_catalog "redhat-operators" "openshift-marketplace" 120
  _resolve_target_csv "redhat-operators" "$channel" 120
  apply_subscription "$channel" "redhat-operators"
}

# Install from the custom index image on the given channel (disables default sources).
# Sets TARGET_CSV to the channel's currentCSV.
install_from_custom() {
  local index_image="$1" channel="$2"
  info "Installing ACS Operator from custom index, channel ${channel}..."
  disable_default_sources
  apply_custom_catalog "$index_image"
  wait_for_catalog "my-operator-catalog"
  _resolve_target_csv "my-operator-catalog" "$channel"
  apply_subscription "$channel" "my-operator-catalog"
}

# Upgrade by refreshing the custom CatalogSource and pointing the subscription at it.
# Sets TARGET_CSV to the channel's currentCSV.
upgrade_via_custom() {
  local index_image="$1" channel="$2"
  info "Upgrading via custom catalog (channel: ${channel})..."
  disable_default_sources
  apply_custom_catalog "$index_image"
  wait_for_catalog "my-operator-catalog"
  _resolve_target_csv "my-operator-catalog" "$channel"
  apply_subscription "$channel" "my-operator-catalog"
  ok "Subscription updated — OLM will upgrade to ${TARGET_CSV}"
}

# Upgrade to latest official GA via redhat-operators.
# Sets TARGET_CSV to the latest channel's currentCSV.
upgrade_to_latest_official() {
  local major="${1:-4}"
  enable_default_sources
  wait_for_catalog "redhat-operators" "openshift-marketplace" 180
  local latest_minor
  latest_minor=$(get_latest_official_minor "$major") || true
  [[ -n "$latest_minor" ]] || fail "Could not determine latest GA minor from redhat-operators"
  local channel="rhacs-${major}.${latest_minor}"
  info "Upgrading to latest GA channel: ${channel}..."
  _resolve_target_csv "redhat-operators" "$channel" 180
  apply_subscription "$channel" "redhat-operators"
  ok "Subscription updated to ${channel}, target: ${TARGET_CSV}"
}

get_current_csv() {
  oc get csv -n openshift-operators --no-headers 2>/dev/null \
    | awk '/rhacs-operator/ && /Succeeded/ {print $1}' \
    | head -1
}

# Waits for a specific CSV to reach Succeeded phase.
# Use after install/upgrade to wait for the exact target version, not just any change.
# This correctly handles multi-hop OLM upgrade graphs (e.g. 4.8→4.9→4.10).
wait_for_csv() {
  local target_csv="$1" timeout="${2:-600}"
  local deadline
  deadline=$(( $(date +%s) + timeout ))
  info "Waiting for ${target_csv} to reach Succeeded (timeout: ${timeout}s)..."
  while (( $(date +%s) < deadline )); do
    local phase
    phase=$(oc get csv "$target_csv" -n openshift-operators \
      -o jsonpath='{.status.phase}' 2>/dev/null || true)
    if [[ "$phase" == "Succeeded" ]]; then
      ok "${target_csv} is Succeeded"
      return 0
    fi
    # Show in-progress CSVs so logs show the hop chain
    local progress
    progress=$(oc get csv -n openshift-operators --no-headers 2>/dev/null \
      | awk '/rhacs-operator/ {printf "%s(%s) ", $1, $7}' || true)
    info "  ${progress:+CSVs: ${progress}}phase=${phase:-not found}, waiting 15s..."
    sleep 15
  done
  fail "${target_csv} did not reach Succeeded within ${timeout}s"
}

log_csv() {
  local csv
  csv=$(oc get csv -n openshift-operators --no-headers 2>/dev/null \
    | grep rhacs-operator || echo "<none>")
  info "Current CSV: ${csv}"
}

# Reset between tests
reset_operator() {
  info "Resetting operator state for next test..."
  oc delete subscription rhacs-operator -n openshift-operators --ignore-not-found
  oc get csv -n openshift-operators --no-headers 2>/dev/null \
    | awk '/rhacs-operator/ {print $1}' \
    | xargs -r oc delete csv -n openshift-operators --ignore-not-found || true
  oc delete catalogsource my-operator-catalog -n openshift-marketplace --ignore-not-found || true
  enable_default_sources
  ok "Operator state reset"
}
