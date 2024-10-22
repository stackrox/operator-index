#!/bin/bash

# The version-controled catalog template in common/catalog-template.json is
# currently auto-generated from the production catalog (in turn built by
# CPaaS/IIB). This makes it hard to modify (because a refresh would wipe manual
# modifications).
#
# This script:
# 1. creates a temporary catalog template from the version controled template
# by injecting a bundle for a Konflux build, together with a rhacs-4.6 channel
# 2. renders this template into a version-controled catalog.
#
# TODO: Once we stop building operator indexes using CPaaS/IIB, the entries for
# the Konflux build can be moved into the version-controled template and the
# script can be removed in favor of the render-template command.

set -euo pipefail

# A recent (more or less) successful konflux build.
version="v4.6.0-719-g818ca4e0b4-fast"

tmp_template="$(mktemp)"
trap 'rm -f $tmp_template' EXIT
jq --slurpfile channel channel-4.6.json '.entries += $channel
 | .entries += [{"schema": "olm.bundle", "image": "quay.io/rhacs-eng/stackrox-operator-bundle:'${version}'"}]
 | .entries |= map(
   if .schema == "olm.channel" and .name == "stable"
   then .entries += [{"name": "rhacs-operator.'${version}'", "replaces": "rhacs-operator.v4.5.3", "skipRange": ">= 4.5.0 < 4.6.0"}]
   else .
   end)
 | .entries |= map(
   if .schema == "olm.channel" and .name == "rhacs-4.6"
   then .entries += [{"name": "rhacs-operator.'${version}'", "replaces": "rhacs-operator.v4.5.0", "skipRange": ">= 4.5.0 < 4.6.0"}]
   else .
   end)
' common/catalog-template.json > "$tmp_template"
echo >&2 "Running template rendering, this can take a few minutes..."
opm alpha render-template basic "$@" "${tmp_template}"
