module github.com/elastic/package-registry/cmd/distribution

go 1.26.0

require (
	github.com/Masterminds/semver/v3 v3.5.0
	github.com/ProtonMail/go-crypto v1.4.1
	github.com/elastic/go-licenser v0.4.2
	github.com/elastic/package-registry v1.38.0
	github.com/google/go-querystring v1.2.0
	github.com/stretchr/testify v1.11.1
	golang.org/x/tools v0.45.0
	gopkg.in/yaml.v3 v3.0.1
	honnef.co/go/tools v0.7.0
)

require (
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/cloudflare/circl v1.6.3 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	golang.org/x/crypto v0.52.0 // indirect
	golang.org/x/exp/typeparams v0.0.0-20231108232855-2478ac86f678 // indirect
	golang.org/x/mod v0.36.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/telemetry v0.0.0-20260508192327-42602be52be6 // indirect
)

replace github.com/elastic/package-registry => ../..
