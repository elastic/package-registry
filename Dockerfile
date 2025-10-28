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

RUN make release-${TARGETPLATFORM:-linux}


# Run binary
FROM ${RUNNER_IMAGE}

# Get dependencies
# Mailcap is installed to get mime types information.
RUN if grep -q "Red Hat" /etc/os-release ; then \
    microdnf install -y mailcap zip rsync && \
    microdnf clean all ; \
  else \
    apk update && \
    apk add mailcap zip rsync curl && \
    rm -rf /var/cache/apk/* ; \
  fi

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
