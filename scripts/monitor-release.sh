#!/bin/bash
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd)"
source "$SCRIPT_DIR/helpers.sh"

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

if ! COMMIT=$(expand_commit "$COMMIT"); then
    exit 1
fi

# Set the interval for checking releases in seconds
interval=10
shift
echo -e "Release status for \033[0;32m$COMMIT\033[0m commit (Press ctrl+C to quit):"
while true; do
    tput reset # Clear the terminal screen
    date  # Show current timestamp
    kubectl -n rh-acs-tenant get releases.appstudio.redhat.com -l pac.test.appstudio.openshift.io/sha="${COMMIT}"
    echo -e "\nChecking again in \033[0;33m$interval\033[0m seconds..."
    echo "(Press ctrl+C to quit)"
    sleep "$interval"
done
