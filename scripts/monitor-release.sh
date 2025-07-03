#!/bin/bash

set -euo pipefail

if [[ "$#" -lt 1 || "$#" -gt 2 ]]; then
    echo "USAGE: ./monitor-release.sh <RELEASE_NAME>"
    echo ""
    echo "RELEASE_NAME - name of the release. You can find it in the metadata.name of the generated .yaml for the release."
    echo ""
    echo "You must have your KUBECONFIG point to the Konflux cluster, see https://spaces.redhat.com/pages/viewpage.action?pageId=407312060#HowtoeverythingKonflux/RHTAPforRHACS-GettingocCLItoworkwithKonflux."
    exit 1
fi

RELEASE_NAME="$1"
    
echo "Release status for ${RELEASE_NAME}"
kubectl get releases | grep "${RELEASE_NAME}"
