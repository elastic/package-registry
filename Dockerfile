# This Dockerfile allows to build the package-registry and packages together.
# It is decoupled from the packages and the registry even though for now both are in the same repository.
ARG GO_VERSION=1.14.2
FROM golang:${GO_VERSION}

# Get dependencies
RUN \
    apt-get update \
      && apt-get install -y --no-install-recommends zip rsync \
      && rm -rf /var/lib/apt/lists/*

# Copy the package registry
COPY ./ /home/package-registry
WORKDIR /home/package-registry

ENV GO111MODULE=on
RUN go get -u github.com/magefile/mage
# Prepare all the packages to be built
RUN mage build

# Build binary
RUN go build .

# Move all files need to run to its own directory
# This will become useful for staged builds later on
RUN mkdir -p /registry/public # left for legacy purposes
RUN mkdir -p /registry/packages/package-storage
RUN mv package-registry /registry/
RUN cp -r build/package-storage/packages/* /registry/packages/package-storage/
RUN cp config.docker.yml /registry/config.yml

# Change to new working directory
WORKDIR /registry

# Clean up files not needed
RUN rm -rf /home/package-registry

EXPOSE 8080

ENTRYPOINT ["./package-registry"]
# Make sure it's accessible from outside the container
CMD ["--address=0.0.0.0:8080"]

HEALTHCHECK --interval=1s --retries=30 CMD curl --silent --fail localhost:8080/health || exit 1
