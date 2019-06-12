FROM golang:latest

COPY ./ /go/src/github.com/elastic/integrations-registry
EXPOSE 8080

WORKDIR "/go/src/github.com/elastic/integrations-registry"
RUN GO111MODULE=on go mod vendor

# Make sure it's accessible from outside the container
CMD ["go", "run", "main.go", "--address=0.0.0.0:8080"]
