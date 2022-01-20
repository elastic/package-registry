# This image contains the package-registry binary.
# It expects packages to be mounted under /packages/package-registry or have a config file loaded into /package-registry/config.yml

# Build binary
ARG GO_VERSION=1.17.6
FROM golang:${GO_VERSION} AS builder

ENV GO111MODULE=on
COPY ./ /package-registry
WORKDIR /package-registry
RUN go build .


# Run binary
FROM centos:7

# Get dependencies
# mailcap - installs "/etc/mime.types" used by the package-registry binary
RUN yum install -y zip rsync mailcap && yum clean all

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

