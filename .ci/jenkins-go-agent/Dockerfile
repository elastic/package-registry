FROM golang:1.12

RUN go get -u github.com/magefile/mage \
  && go get -u github.com/elastic/go-licenser \
  && go get -u -v golang.org/x/tools/cmd/goimports

ENV GO111MODULE="auto"
