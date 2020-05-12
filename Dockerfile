# This Dockerfile allows to build the package-registry and packages together.
# It is decoupled from the packages and the registry even though for now both are in the same repository.
ARG GO_VERSION=1.14.2
FROM golang:${GO_VERSION}

# Get dependencies
RUN \
    apt-get update \
      && apt-get install -y --no-install-recommends \
         zip rsync \
      && rm -rf /var/lib/apt/lists/*

# Check out package storage
WORKDIR /home
RUN git clone https://github.com/elastic/package-storage.git
WORKDIR /home/package-storage
ARG PACKAGE_STORAGE_REVISION=master
RUN git checkout ${PACKAGE_STORAGE_REVISION}
LABEL package-storage-revision=${PACKAGE_STORAGE_REVISION}

# Copy the package registry
COPY ./ /home/package-registry
WORKDIR /home/package-registry
RUN mkdir -p /home/package-registry/dev/packages/storage
RUN rsync -av /home/package-storage/packages/ /home/package-registry/dev/packages/storage

ENV GO111MODULE=on
RUN go mod vendor
RUN go get -u github.com/magefile/mage
# Prepare all the packages to be built
RUN mage build

# Build binary
RUN go build .

# Move all files need to run to its own directory
# This will become useful for staged builds later on
RUN mkdir /registry
RUN mv package-registry /registry/
RUN mv config.yml /registry/
RUN mv public /registry/

# Change to new working directory
WORKDIR /registry

# Clean up files not needed
RUN rm -rf /home/package-registry
RUN rm -rf /home/package-storage

EXPOSE 8080

ENTRYPOINT ["./package-registry"]
# Make sure it's accessible from outside the container
CMD ["--address=0.0.0.0:8080"]

HEALTHCHECK --interval=1s --retries=30 CMD curl --silent --fail localhost:8080/health || exit 1
