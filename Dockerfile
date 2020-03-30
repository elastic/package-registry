ARG GO_VERSION
FROM golang:${GO_VERSION:-1.13.4}

RUN \
    apt-get update \
      && apt-get install -y --no-install-recommends \
         zip rsync \
      && rm -rf /var/lib/apt/lists/*

COPY ./ /go/src/github.com/elastic/package-registry
EXPOSE 8080

WORKDIR "/go/src/github.com/elastic/package-registry"

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
