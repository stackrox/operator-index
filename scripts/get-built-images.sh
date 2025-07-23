#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd)"
source "$SCRIPT_DIR/helpers.sh"

if [[ "$#" -gt 1 || "${1:-}" == "--help" ]]; then
    echo "USAGE: ./$(basename "${BASH_SOURCE[0]}") [COMMIT]"
    echo ""
    echo "COMMIT - an optional 40 character-long SHA of the commit to pull built images only with this commit sha. Default: the latest commit in the current branch"
    echo ""
    echo "You must have your KUBECONFIG point to the Konflux cluster, see https://spaces.redhat.com/pages/viewpage.action?pageId=407312060#HowtoeverythingKonfluxforRHACS-GettingocCLItoworkwithKonflux."
    exit 1
fi

COMMIT="${1:-$(git rev-parse HEAD)}"
COMMIT="$(expand_commit "$COMMIT")"

echo -e "Operator catalog images for commit \033[0;32m$COMMIT\033[0m:"
result="$(kubectl get -n rh-acs-tenant snapshot -l pac.test.appstudio.openshift.io/sha="${COMMIT}" -o jsonpath='{range .items[*].spec.components[?(@.containerImage)]}{.name}: {.containerImage}{"\n"}{end}')"
: "${result:?Error: No Snapshot CRs found for the commit. Use Konflux UI to get built images instead: https://konflux-ui.apps.stone-prd-rh01.pg1f.p1.openshiftapps.com/ns/rh-acs-tenant/applications.}"
echo "$result"
