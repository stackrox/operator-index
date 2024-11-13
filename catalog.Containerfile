ARG base_image
FROM ${base_image}

ARG catalog_dir
RUN echo "Checking required catalog_dir"; [[ "${catalog_dir}" != "" ]]

ENTRYPOINT ["/bin/opm"]
CMD ["serve", "/configs", "--cache-dir=/tmp/cache"]

COPY ${catalog_dir}/ /configs
LABEL operators.operatorframework.io.index.configs.v1=/configs

# Build the cache such that we can use this image as the target for a standalone CatalogSource.
RUN ["/bin/opm", "serve", "/configs", "--cache-dir=/tmp/cache", "--cache-only"]
