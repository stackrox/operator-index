#!/bin/bash

set -euo pipefail

if [[ "$#" -lt 2 || "$#" -gt 3 ]]; then
    echo "USAGE: ./generate-releases.sh <ENVIRONMENT> <RELEASE_NAME_SUFFIX> [<OPERATOR_INDEX_COMMIT>]"
    echo ""
    echo "ENVIRONMENT - allowed values: staging|prod"
    echo "RELEASE_NAME_SUFFIX - for production, use something like acs-4-6-x-1; for staging acs-4-6-x-staging-1"
    echo "OPERATOR_INDEX_COMMIT - default: currently checked out commit"
    echo ""
    echo "You must have your KUBECONFIG point to the Konflux cluster, see https://spaces.redhat.com/pages/viewpage.action?pageId=407312060#HowtoeverythingKonflux/RHTAPforRHACS-GettingocCLItoworkwithKonflux."
    exit 1
fi

ENVIRONMENT="$1"
RELEASE_NAME_SUFFIX="$2"
OPERATOR_INDEX_COMMIT="${3:-$(git rev-parse HEAD)}"

validate_input() {
    if [ "$(kubectl get snapshot -l pac.test.appstudio.openshift.io/sha="${OPERATOR_INDEX_COMMIT}" --no-headers | wc -l)" -eq 0 ]; then
        echo "ERROR: Could not find any Snapshots for the commit '${OPERATOR_INDEX_COMMIT}'. This must be a 40 character-long commit SHA. Default: currently checked out commit."
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
