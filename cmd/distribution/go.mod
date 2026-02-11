module github.com/elastic/package-registry/cmd/distribution

go 1.24.0

require (
	github.com/Masterminds/semver/v3 v3.4.0
	github.com/ProtonMail/go-crypto v1.3.0
	github.com/elastic/package-registry v0.0.0
	github.com/google/go-querystring v1.1.0
	github.com/stretchr/testify v1.11.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/cloudflare/circl v1.6.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	golang.org/x/crypto v0.48.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
)

replace github.com/elastic/package-registry => ../..
