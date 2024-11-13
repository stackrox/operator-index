CATALOGS = catalog-bundle-object/rhacs-operator/catalog.json catalog-csv-metadata/rhacs-operator/catalog.json

MAKEFLAGS += "-j 2"

.PHONY: valid-catalogs clean import-legacy
valid-catalogs: $(CATALOGS)
	opm validate catalog-bundle-object
	opm validate catalog-csv-metadata

clean:
	rm -f $(CATALOGS)
	rm -rf catalog-migrate

catalog-bundle-object/rhacs-operator/catalog.json: catalog-template.json render-template.sh
	mkdir -p "$$(dirname "$@")"
	./render-template.sh --migrate-level none > $@

catalog-csv-metadata/rhacs-operator/catalog.json: catalog-template.json render-template.sh
	mkdir -p "$$(dirname "$@")"
	./render-template.sh --migrate-level bundle-object-to-csv-metadata > $@

# This is broken due to concurrency if invoked together with other targets (e.g. `make import-legacy valid-catalogs` - don't do this).
# Instead invoke `make import-legacy && make valid-catalogs`.
# TODO: fix it. Otherwise this target will disappear once konflux index builds replace the CPaaS-based ones.
import-legacy:
	opm migrate registry.redhat.io/redhat/redhat-operator-index:v4.12 ./catalog-migrate
	opm alpha convert-template basic ./catalog-migrate/rhacs-operator/catalog.json > catalog-template.json
