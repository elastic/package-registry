FROM golang:1.12

RUN \
    apt-get update \
      && apt-get install -y --no-install-recommends \
         zip \
      && rm -rf /var/lib/apt/lists/*

RUN go get -u github.com/magefile/mage \
  && go get -u github.com/elastic/go-licenser \
  && go get -u -v golang.org/x/tools/cmd/goimports

ENV GO111MODULE="auto"
