# This image contains the package-registry binary.
# It expects packages to be mounted under /packages/package-registry or have a config file loaded into /package-registry/config.yml

# Build binary
ARG GO_VERSION=1.20.2
FROM golang:${GO_VERSION} AS builder

COPY ./ /package-registry
WORKDIR /package-registry
RUN go build .


# Run binary
FROM ubuntu:22.04

# Get dependencies
RUN apt-get update && \
    apt-get install -y media-types zip rsync curl && \
    rm -rf /var/lib/apt/lists/*

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

