# This Dockerfile allows to build the package-registry and packages together.
# It is decoupled from the packages and the registry even though for now both are in the same repository.
ARG GO_VERSION=1.14.2
FROM golang:${GO_VERSION}

RUN \
    apt-get update \
      && apt-get install -y --no-install-recommends \
         zip rsync \
      && rm -rf /var/lib/apt/lists/*

RUN go get github.com/elastic/package-registry
WORKDIR "/go/src/github.com/elastic/package-registry"

# Define which version of the packages should be checked out.
ARG PACKAGES_VERSION=master
RUN git checkout ${PACKAGES_VERSION}
EXPOSE 8080

ENV GO111MODULE=on
RUN go mod vendor
RUN go get -u github.com/magefile/mage
RUN mage build

# This directory contains the packages at the moment but is only used during the build process
# If we keep it, it means all packages exist twice.
RUN rm -rf dev

ENTRYPOINT ["go", "run", "."]
# Make sure it's accessible from outside the container
CMD ["--address=0.0.0.0:8080"]

HEALTHCHECK --interval=1s --retries=30 CMD curl --silent --fail localhost:8080/ || exit 1
