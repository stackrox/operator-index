# ACS Operator Index

This repository is for building the ACS (downstream) operator indexes on Konflux.

**Note: [opm](https://github.com/operator-framework/operator-registry/releases) v1.46.0 or newer is required to manipulate the files here.**

## Initialization

Followed [this](https://gitlab.cee.redhat.com/konflux/docs/users/-/blob/main/topics/getting-started/building-olm-products.md)
and [this](https://github.com/konflux-ci/olm-operator-konflux-sample/blob/main/docs/konflux-onboarding.md) doc:
1. Fetched a recent OPM
2. `make import-legacy && make valid-catalogs`

## Catalog formats

This directory contains two versions of the catalog, in subdirectories `catalog-bundle-object` and `catalog-csv-metadata`.
The former is expected by OpenShift versions up to and including 4.16, and the latter - by 4.17 and later.

See [konflux docs](https://github.com/konflux-ci/build-definitions/blob/c93ea73dbc30b8be15615e4d230040c70a0cf826/task/fbc-validation/0.1/TROUBLESHOOTING.md?plain=1#L7-L8).
