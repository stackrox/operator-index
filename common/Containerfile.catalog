ARG base_image
FROM ${base_image}

ENTRYPOINT ["/bin/opm"]
CMD ["serve", "/configs", "--cache-dir=/tmp/cache"]

COPY catalog/ /configs
LABEL operators.operatorframework.io.index.configs.v1=/configs

# Build the cache such that we can use this image as the target for a standalone CatalogSource.
RUN ["/bin/opm", "serve", "/configs", "--cache-dir=/tmp/cache", "--cache-only"]
