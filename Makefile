CATALOGS = common/catalog/rhacs-operator/catalog.json common/catalog-csv-metadata/rhacs-operator/catalog.json

.PHONY: all clean
all: $(CATALOGS)
clean:
	rm -f $(CATALOGS)

common/catalog/rhacs-operator/catalog.json: common/catalog-template.json
	opm alpha render-template basic --migrate-level none $< > $@

common/catalog-csv-metadata/rhacs-operator/catalog.json: common/catalog-template.json
	opm alpha render-template basic --migrate-level bundle-object-to-csv-metadata $< > $@
