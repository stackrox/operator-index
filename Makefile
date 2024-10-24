CATALOGS = common/catalog-bundle-object/rhacs-operator/catalog.json common/catalog-csv-metadata/rhacs-operator/catalog.json

MAKEFLAGS += "-j 2"

.PHONY: valid-catalogs clean import-legacy
valid-catalogs: $(CATALOGS)
	opm validate common/catalog-bundle-object
	opm validate common/catalog-csv-metadata

clean:
	rm -f $(CATALOGS)
	rm -rf catalog-migrate

common/catalog-bundle-object/rhacs-operator/catalog.json: common/catalog-template.json render-template.sh
	mkdir -p "$$(dirname "$@")"
	./render-template.sh --migrate-level none > $@

common/catalog-csv-metadata/rhacs-operator/catalog.json: common/catalog-template.json render-template.sh
	mkdir -p "$$(dirname "$@")"
	./render-template.sh --migrate-level bundle-object-to-csv-metadata > $@

import-legacy:
	opm migrate registry.redhat.io/redhat/redhat-operator-index:v4.12 ./catalog-migrate
	opm alpha convert-template basic ./catalog-migrate/rhacs-operator/catalog.json > common/catalog-template.json
