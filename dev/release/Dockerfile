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

WORKDIR /home

# Check out package storage
RUN git clone https://github.com/elastic/package-storage.git
WORKDIR /home/package-storage
ARG PACKAGE_STORAGE_REVISION=master
RUN git checkout ${PACKAGE_STORAGE_REVISION}
LABEL package-storage-revision=${PACKAGE_STORAGE_REVISION}

WORKDIR /home

# Checkout out package registry
RUN git clone https://github.com/elastic/package-registry.git
WORKDIR "/home/package-registry"
ARG PACKAGE_REGISTRY_REVISION=master
RUN git checkout ${PACKAGE_REGISTRY_REVISION}
LABEL package-registry-revision=${PACKAGE_REGISTRY_REVISION}

# Get package storage packages into registry
# Note: In the future, packages will only be in the storage and not the registry
RUN rsync -a /home/package-storage/packages/ /home/package-registry/dev/packages/example

EXPOSE 8080

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

# Clean up files not needed
RUN rm -rf /go/src/github.com/elastic/package-registry
RUN rm -rf /go/package-storage

# Change to new working directory
WORKDIR /registry

ENTRYPOINT ["./package-registry"]
# Make sure it's accessible from outside the container
CMD ["--address=0.0.0.0:8080"]

HEALTHCHECK --interval=1s --retries=30 CMD curl --silent --fail localhost:8080/health || exit 1
