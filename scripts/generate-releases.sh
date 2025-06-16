#!/bin/bash

set -euo pipefail

if [[ "$#" -lt 2 || "$#" -gt 4 ]]; then
    echo "USAGE: ./generate-releases.sh <ENVIRONMENT> <RELEASE_NAME_SUFFIX> [<OPERATOR_INDEX_COMMIT>] [<OPERATOR_INDEX_BRANCH>]"
    echo ""
    echo "ENVIRONMENT - allowed values: staging|prod"
    echo "RELEASE_NAME_SUFFIX - for production, use something like acs-4-6-x-1; for staging acs-4-6-x-staging-1"
    echo "OPERATOR_INDEX_COMMIT - default: currently checked out commit"
    echo "OPERATOR_INDEX_BRANCH - default: currently checked branch"
    echo ""
    echo "You must have your KUBECONFIG point to the Konflux cluster, see https://spaces.redhat.com/pages/viewpage.action?pageId=407312060#HowtoeverythingKonflux/RHTAPforRHACS-GettingocCLItoworkwithKonflux."
    exit 1
fi

ENVIRONMENT="$1"
RELEASE_NAME_SUFFIX="$2"
OPERATOR_INDEX_COMMIT="${3:-$(git rev-parse HEAD)}"
OPERATOR_INDEX_BRANCH="${4:-$(git rev-parse --abbrev-ref HEAD)}"
snapshot_list="$(kubectl get snapshot -l pac.test.appstudio.openshift.io/sha="${OPERATOR_INDEX_COMMIT}" -o json | jq -r '.items[] | select(.metadata.annotations["pac.test.appstudio.openshift.io/branch"]=="'"${OPERATOR_INDEX_BRANCH}"'") | "\(.metadata.name)|\(.spec.application)"')"

validate_input() {
    supported_ocp_number=$(find ".tekton" -maxdepth 1 -type f -name "operator-index-ocp-*-build.yaml" | wc -l | xargs)
    snapshot_number=$(echo "$snapshot_list" | wc -l | xargs)

    if [ "$snapshot_number" -eq 0 ]; then
        echo "ERROR: Could not find any Snapshots for the commit '${OPERATOR_INDEX_COMMIT}'. This must be a 40 character-long commit SHA. Default: currently checked out commit." >&2
        exit 1
    fi
    if [[ "${ENVIRONMENT}" != "staging" && "${ENVIRONMENT}" != "prod" ]]; then
        echo "ERROR: ENVIRONMENT input must either be 'staging' or 'prod'." >&2
        exit 1
    fi
    if [[ "${ENVIRONMENT}" == "prod" && "${OPERATOR_INDEX_BRANCH}" != "master" ]]; then
        echo "ERROR: prod release has to be done on master branch" >&2
        exit 1
    fi
    if [[ "$snapshot_number" -ne "$supported_ocp_number" ]]; then
        echo "snapshot list:" >&2
        echo "$snapshot_list" >&2
        echo "ERROR: The number of snapshots for $OPERATOR_INDEX_COMMIT in branch $OPERATOR_INDEX_BRANCH does not match the number of supported OCP versions." >&2
        exit 1
    fi
}

generate_release_resources() {
    for d in $snapshot_list; do
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
