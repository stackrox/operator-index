#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd)"
source "$SCRIPT_DIR/helpers.sh"

# Check command-line arguments and display usage help, if needed.
usage() {
    if [[ "$#" -lt 1 || "$#" -gt 3 || "${1:-}" == "--help" ]]; then
        echo "USAGE: ./generate-releases.sh <ENVIRONMENT> [COMMIT] [BRANCH]" >&2
        echo "" >&2
        echo "ENVIRONMENT - allowed values: staging|prod" >&2
        echo "COMMIT - a SHA of the commit to pull Snapshots only with this commit label for the Release." >&2
        echo "If provided commit SHA is less than 40 characters then it will be expanded to a full 40-characters long SHA. Default: currently checked out commit" >&2
        echo "BRANCH - an optional parameter to specify git branch name for filtering snapshots by having branch name in annotations. Default: currently checked out branch" >&2
        echo "" >&2
        echo "You must have your KUBECONFIG point to the Konflux cluster, see https://spaces.redhat.com/pages/viewpage.action?pageId=407312060#HowtoeverythingKonfluxforRHACS-GettingocCLItoworkwithKonflux." >&2
        return 1
    fi
}

# Validate environment input and ensure it is either 'staging' or 'prod' (for master branch only).
validate_environment() {
    local -r environment="$1"
    local -r branch="$2"
    if [[ "${environment}" != "staging" && "${environment}" != "prod" ]]; then
        echo "ERROR: ENVIRONMENT input must either be 'staging' or 'prod'." >&2
        return 1
    fi
    if [[ "${environment}" == "prod" && "${branch}" != "master" ]]; then
        echo "ERROR: prod release has to be done on master branch" >&2
        return 1
    fi
}

# Fetch the list of snapshots for provided commit and branch values. 
# Make sure that only one the most recent snapshot per application is returned.
get_snapshots() {
    local -r commit="$1"
    local -r branch="$2"
    kubectl get -n rh-acs-tenant snapshot -l pac.test.appstudio.openshift.io/sha="${commit}" -o json | jq -r '
        .items
        | map(select((.metadata.annotations["pac.test.appstudio.openshift.io/source-branch"]=="'"${branch}"'") or (.metadata.annotations["pac.test.appstudio.openshift.io/source-branch"]=="refs/heads/'${branch}'")))
        | sort_by(.spec.application)
        | group_by(.spec.application)
        | map(sort_by(.metadata.creationTimestamp) | last)
        | .[]
        | "\(.metadata.name)|\(.spec.application)"'
}

# Validate all expected Snapshots were found.
validate_snapshots() {
    local -r commit="$1"
    local -r snapshot_list="$2"

    local pipelines_count
    local snapshots_count
    pipelines_count="$(find ".tekton" -maxdepth 1 -type f -name "operator-index-ocp-*-build.yaml" | wc -l )"
    snapshots_count="$(echo "$snapshot_list" | wc -l )"

    echo -e "Found the following snapshots for \033[0;32m$commit\033[0m commit:" >&2
    echo "$snapshot_list" >&2

    if [[ "$snapshots_count" -eq 0 ]]; then
        echo "ERROR: Could not find any Snapshots for the commit '${commit}'." >&2
        return 1
    fi
    if [[ "$snapshots_count" -ne "$pipelines_count" ]]; then
        echo "ERROR: The number of snapshots $snapshots_count for $commit does not match the number of supported OCP versions $pipelines_count." >&2
        return 1
    fi
}

# Generate the Release resources for each snapshot found.
generate_release_resources() {
    local -r environment="$1"
    local -r snapshot_list="$2"

    local release_name
    release_name="${environment}-$(git rev-parse --short HEAD)-$(date +'%Y-%m-%d-%H-%M')"

    local snapshot
    local application
    local release_plan
    
    while IFS= read -r line
    do
        snapshot="$(echo "$line" | cut -d "|" -f 1)"
        application="$(echo "$line" | cut -d "|" -f 2)"
        release_plan="${application/acs-operator-index/acs-operator-index-${environment}}"
        echo "---
apiVersion: appstudio.redhat.com/v1alpha1
kind: Release
metadata:
  name: ${application}-${release_name}
  namespace: rh-acs-tenant
spec:
  releasePlan: ${release_plan}
  snapshot: ${snapshot}"
    done <<< "$snapshot_list"
}

usage "$@"

environment="$1"

commit="${2:-$(git rev-parse HEAD)}"
commit="$(expand_commit "$commit")"

branch="${3:-$(git rev-parse --abbrev-ref HEAD)}"
validate_branch "$branch"

validate_environment "$environment" "$branch"

snapshot_list=$(get_snapshots "$commit" "$branch")
validate_snapshots "$commit" "$snapshot_list"

generate_release_resources "$environment" "$snapshot_list"
