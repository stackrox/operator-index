#!/bin/bash

set -euo pipefail

if [[ "$#" -gt 1 ]]; then
    echo "USAGE: ./monitor-release.sh [COMMIT]"
    echo ""
    echo "COMMIT - an optional 40 character-long SHA of the commit to pull releases CRs associeted with this commit. Default: the latest commit in the current branch"
    echo ""
    echo "You must have your KUBECONFIG point to the Konflux cluster, see https://spaces.redhat.com/pages/viewpage.action?pageId=407312060#HowtoeverythingKonfluxforRHACS-GettingocCLItoworkwithKonflux."
    exit 1
fi

COMMIT="${1:-$(git rev-parse HEAD)}"

# Check KUBECONFIG is set to Konflux cluster and correct project/namespace is selected
if [[ "$(kubectl config view --minify --output 'jsonpath={...current-context}')" != "rh-acs-tenant" ]]; then
    echo 'ERROR: Namespace "rh-acs-tenant" is not selected.'
    echo "Make sure you loged in to Konflux cluster: https://oauth-openshift.apps.stone-prd-rh01.pg1f.p1.openshiftapps.com/oauth/token/request"
    echo 'Switch to "rh-acs-tenant" project: oc project rh-acs-tenant'
    exit 1
fi

# Check if COMMIT is a valid 40-character hexadecimal git SHA
if ! [[ "$COMMIT" =~ ^[0-9a-fA-F]{40}$ ]] || ! git cat-file -e "$COMMIT"^{commit} 2>/dev/null; then
    echo "ERROR: Provided COMMIT is not a 40 character-long git commit SHA."
    exit 1
fi
    
echo -e "Release status for \033[0;32m$COMMIT\033[0m commit (Press ctrl+C to quit):"
kubectl -n rh-acs-tenant get releases.appstudio.redhat.com --watch -l pac.test.appstudio.openshift.io/sha="${COMMIT}"
