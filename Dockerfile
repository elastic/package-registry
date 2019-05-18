FROM golang:latest

COPY ./ /go/src/github.com/elastic/integrations-registry
EXPOSE 8080

WORKDIR "/go/src/github.com/elastic/integrations-registry"
RUN GO111MODULE=on go mod vendor
CMD ["go", "run", "main.go"]
