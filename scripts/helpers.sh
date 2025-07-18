# Check KUBECONFIG is set to Konflux cluster.
validate_kubeconfig() {
    current_context="$(kubectl config view --minify --output 'jsonpath={.current-context}')"
    konflux_context="rh-acs-tenant/api-stone-prd-rh01-pg1f-p1-openshiftapps-com:6443"
    if [[ "$current_context" != "$konflux_context"* ]]; then
        echo 'ERROR: Current kubeconfig context does not set to Konflux cluster.' >&2
        echo "Make sure you loged in to Konflux cluster: https://oauth-openshift.apps.stone-prd-rh01.pg1f.p1.openshiftapps.com/oauth/token/request" >&2
        echo 'Switch to "rh-acs-tenant" project: oc project rh-acs-tenant' >&2
        exit 1
    fi
}

# Check if COMMIT is a valid git commit SHA. If it's a short 7-characters SHA, it will be expanded to a full 40-character SHA.
validate_commit() {
    if ! git cat-file -e "$1"^{commit} 2>/dev/null; then
        echo "ERROR: Provided COMMIT is not a known git commit SHA on the repome repository." >&2
        exit 1
    fi
    if [[ ${#1} -eq 7 ]]; then
        COMMIT="$(git rev-parse --verify --end-of-options "$1^{commit}")"
    fi
}

# Check if BRANCH is a valid known branch git branch for the remote repository, otherwise it's unlikely that any Snapshots will be found for it.
validate_branch() {
    if ! git ls-remote --exit-code --heads origin "$1" > /dev/null; then
        echo "ERROR: $1 branch does not exist on remote." >&2
        exit 1
    fi
}