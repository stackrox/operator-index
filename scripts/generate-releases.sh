#!/bin/bash

set -euo pipefail

if [ "$#" -ne 3 ]; then
    echo "USAGE: ./generate-releases.sh <OPERATOR_INDEX_COMMIT> <ENVIRONMENT> <RELEASE_NAME_SUFFIX>"
    exit 1
fi

OPERATOR_INDEX_COMMIT="$1"
ENVIRONMENT="$2"
RELEASE_NAME_SUFFIX="$3"

validate_input() {
    if [ "$(kubectl get snapshot -l pac.test.appstudio.openshift.io/sha="${OPERATOR_INDEX_COMMIT}" --no-headers | wc -l)" -eq 0 ]; then
        echo "ERROR: Could not find any snapshots for the commit '${OPERATOR_INDEX_COMMIT}'."
        exit 1
    fi
    if [[ "${ENVIRONMENT}" != "staging" && "${ENVIRONMENT}" != "prod" ]]; then
        echo "ERROR: ENVIRONMENT input must either be 'staging' or 'prod'."
        exit 1
    fi
}

generate_release_resources() {
    snapshot_data="$(kubectl get snapshot -l pac.test.appstudio.openshift.io/sha="${OPERATOR_INDEX_COMMIT}" -o jsonpath='{range .items[*]}{.metadata.name}|{.spec.application}{"\n"}{end}')"

    for d in $snapshot_data; do
        snapshot="$(echo "$d" | cut -d "|" -f 1)"
        application="$(echo "$d" | cut -d "|" -f 2)"
        release_plan="${application/acs-operator-index/acs-operator-index-${ENVIRONMENT}}"
        echo "---
apiVersion: appstudio.redhat.com/v1alpha1
kind: Release
metadata:
  name: ${application}-${RELEASE_NAME_SUFFIX}
  namespace: rh-acs-tenant
spec:
  releasePlan: ${release_plan}
  snapshot: ${snapshot}"
    done
}

validate_input
generate_release_resources
