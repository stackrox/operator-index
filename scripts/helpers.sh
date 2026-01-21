#!/bin/bash

# Constant for the annotation name of the original snapshot name.
ORIGINAL_SNAPSHOT_ANNOTATION_NAME="acs.redhat.com/original-snapshot-name"

# Expand commit to a full 40-character SHA. Returns the full commit SHA if successful, or an error message if not.
expand_commit() {
    git fetch --all --quiet

    if ! git rev-parse --verify --end-of-options "$1^{commit}"; then
        echo "Cannot expand commit $1 to a full 40-character long SHA." >&2
        return 1
    fi
}

# Check if BRANCH is a valid known branch git branch for the remote repository, otherwise it's unlikely that any Snapshots will be found for it.
# Also check if the provided commit belongs to the branch.
validate_branch() {
    git fetch --all

    if ! git ls-remote --exit-code --heads origin "$1" > /dev/null; then
        echo "ERROR: $1 branch does not exist on remote. Please check the branch name." >&2
        return 1
    fi

    if ! git merge-base --is-ancestor "$2" "$1"; then
        echo "ERROR: commit $2 does not belong to $1 branch. Please check the branch name." >&2
        return 1
    fi
}

# Fetches the list of snapshots for provided commit and branch values.
# Makes sure that only one the most recent snapshot per application is returned.
# Snapshot copies created by generate-releases.sh are filtered out via our custom annotation.
get_snapshots() {
    local -r commit="$1"
    local -r branch="$2"

    kubectl ka get snapshots -n rh-acs-tenant -l pac.test.appstudio.openshift.io/sha="${commit}" -o json | jq '
        .items
        | map(select((.metadata.annotations["pac.test.appstudio.openshift.io/source-branch"]=="'"${branch}"'") or (.metadata.annotations["pac.test.appstudio.openshift.io/source-branch"]=="refs/heads/'${branch}'")))
        | map(select(.metadata.annotations["'"${ORIGINAL_SNAPSHOT_ANNOTATION_NAME}"'"] == null))
        | sort_by(.spec.application)
        | group_by(.spec.application)
        | map(sort_by(.metadata.creationTimestamp)
        | last)'
}
