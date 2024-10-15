# ACS Operator Index

This repository is for building the ACS (downstream) operator indexes on Konflux.

**Note: [opm](https://github.com/operator-framework/operator-registry/releases) v1.46.0 or newer is required to manipulate the files here.**

## Initialization

Followed [this](https://gitlab.cee.redhat.com/konflux/docs/users/-/blob/main/topics/getting-started/building-olm-products.md)
and [this](https://github.com/konflux-ci/olm-operator-konflux-sample/blob/main/docs/konflux-onboarding.md) doc:
1. Fetched a recent OPM
2. `opm migrate registry.redhat.io/redhat/redhat-operator-index:v4.12 ./catalog-migrate`
3. `mkdir -p common/catalog/rhacs-operator`
4. `opm alpha convert-template basic ./catalog-migrate/rhacs-operator/catalog.json > common/catalog-template.json`
5. `opm alpha render-template basic common/catalog-template.json > common/catalog/rhacs-operator/catalog.json`
6. `opm validate common/catalog`
