module github.com/elastic/package-registry

go 1.12

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/elastic/go-ucfg v0.8.3
	github.com/gorilla/mux v1.7.2
	github.com/jstemmer/go-junit-report v0.9.1 // indirect
	github.com/magefile/mage v1.9.0
	github.com/pkg/errors v0.8.1
	github.com/stretchr/testify v1.4.0
	gopkg.in/yaml.v2 v2.2.2
)

replace github.com/elastic/go-ucfg => github.com/mtojek/go-ucfg v0.8.4-0.20200409161607-b87b280107a8
