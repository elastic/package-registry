# This image contains the package-registry binary.
# It expects packages to be mounted under /packages/package-registry or have a config file loaded into /package-registry/config.yml

ARG GO_VERSION
ARG BUILDER_IMAGE=golang
ARG RUNNER_IMAGE=cgr.dev/chainguard/wolfi-base

# Build binary
FROM --platform=${BUILDPLATFORM:-linux} ${BUILDER_IMAGE}:${GO_VERSION} AS builder

COPY ./ /package-registry
WORKDIR /package-registry

ARG TARGETPLATFORM

ENV CGO_ENABLED=1

RUN case "${TARGETPLATFORM}" in \
    "linux/arm")  apt-get update && apt-get install -y gcc-arm-linux-gnueabihf && apt-get clean && rm -rf /var/lib/apt/lists/* ;; \
    "linux/arm64") apt-get update && apt-get install -y gcc-aarch64-linux-gnu && apt-get clean && rm -rf /var/lib/apt/lists/* ;; \
    "linux/amd64") true ;; \
    *) exit 1 ;; \
  esac

RUN case "${TARGETPLATFORM}" in \
    "linux/arm") CC=arm-linux-gnueabihf-gcc CXX=arm-linux-gnueabihf-g++ make release-${TARGETPLATFORM:-linux} ;; \
    "linux/arm64") CC=aarch64-linux-gnu-gcc CXX=aarch64-linux-gnu-g++ make release-${TARGETPLATFORM:-linux} ;; \
    "linux/amd64") make release-${TARGETPLATFORM:-linux} ;; \
    *) exit 1 ;; \
  esac


# Run binary
FROM ${RUNNER_IMAGE}

# Get dependencies
# Mailcap is installed to get mime types information.
RUN apk update && \
    apk add mailcap zip rsync curl && \
    rm -rf /var/cache/apk/*

# Move binary from the builder image
COPY --from=builder /package-registry/package-registry /package-registry/package-registry

# Change to new working directory
WORKDIR /package-registry

# Get in config which expects packages in /packages
COPY config.docker.yml /package-registry/config.yml

# Start registry when container is run an expose it on port 8080
EXPOSE 8080
ENTRYPOINT ["./package-registry"]

# Make sure it's accessible from outside the container
ENV EPR_ADDRESS=0.0.0.0:8080

HEALTHCHECK --interval=1s --retries=30 CMD curl --silent --fail localhost:8080/health || exit 1

