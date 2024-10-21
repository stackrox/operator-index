CATALOGS = common/catalog-bundle-object/rhacs-operator/catalog.json common/catalog-csv-metadata/rhacs-operator/catalog.json

MAKEFLAGS += "-j 2"

.PHONY: valid-catalogs clean
valid-catalogs: $(CATALOGS)
	opm validate common/catalog-bundle-object
	opm validate common/catalog-csv-metadata

clean:
	rm -f $(CATALOGS)

common/catalog-bundle-object/rhacs-operator/catalog.json: common/catalog-template.json
	mkdir -p "$$(dirname "$@")"
	./render-template.sh --migrate-level none > $@

common/catalog-csv-metadata/rhacs-operator/catalog.json: common/catalog-template.json
	mkdir -p "$$(dirname "$@")"
	./render-template.sh --migrate-level bundle-object-to-csv-metadata > $@
