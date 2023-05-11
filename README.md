# Elastic Package Registry (EPR)

## API

Endpoints:

* `/`: Info about the registry
* `/search`: Search for packages. By default returns all the most recent packages available.
* `/categories`: List of the existing package categories and how many packages are in each category.
* `/package/{name}/{version}`: Info about a package
* `/epr/{name}/{name}-{version}.zip`: Download a package

### /search

The `/search` API endpoint has few additional query parameters. More might be added in the future, but for now these are:

* `kibana.version`: Filters out all the packages which are not compatible with the given Kibana version. If it is set to `7.3.1` and
  a package requires 7.4, the package will not be returned or an older compatible package will be shown.
  By default this endpoint always returns only the newest compatible package.
* `category`: Filters the package by the given category. Available categories can be seend when going to `/categories` endpoint.
* `package`: Filters by a specific package name, for example `mysql`. Returns the most recent version.
* `all`: This can be set to `true` to list all package versions. This is set to `false` by default.
* `prerelease`: This can be set to `true` to list prerelease versions of packages. Versions are considered prereleases if they are not stable according to sematic versioning, that is, if they are 0.x versions, or if they contain a prerelease tag. This is set to `false` by default.
* `experimental` (deprecated): This can be set to `true` to list packages considered to be experimental. This is set to `false` by default.

The different query parameters above can be combined, so `?package=mysql&kibana.version=7.3.0` will return all mysql package versions
which are compatible with `7.3.0`.

### /categories

The `/categories` API endpoint has two additional query parameters.

* `prerelease`: This can be set to `true` to list prerelease versions of packages. Versions are considered prereleases if they are not stable according to sematic versioning, that is, if they are 0.x versions, or if they contain a prerelease tag. This is set to `false` by default.
* `experimental` (deprecated): This can be set to `true` to list categories from experimental packages. This is set to `false` by default.
* `include_policy_templates`: This can be set to `true` to include categories from policy templates. This is set to `false` by default.

## Package structure

The package structure has been formalized and described using [package specification](https://github.com/elastic/package-spec).
If you need to modify the structure and corresponding implementation of the Package Registry, remember to adjust the spec first.

## Architecture

There are 2 main parts to the package registry:

* Generation of package and content
* Serving content through a simple http endpoint

As much of the endpoints as possible is generated in advance. All package data is pregenerate and is statically served.
The only exceptions here are the `/categories` and `/search` endpoint as they allow query parameters which leads to many
variations and will also get more complex over time.

`mage build` takes all the packages and generates the content under `public`. The generated content itself is not checked in.

## Directories

* build/packages: Contains all the example packages. These are only example packages used for development. Run `mage build` to generate these.
* testdata/package: Contains the package for testing. This also serves as an example for a package.

## Running

There are several options to run this for development purposes.

### Go command

We recommend using [GVM](https://github.com/andrewkroh/gvm), same as done in the CI.
This tool allows you to install multiple versions of Go, setting the Go environment in consequence: `eval "$(gvm 1.15.9)"`

To use the correct golang version, run:

```
gvm use $(cat .go-version)
```

When using the go command, first the example packages must be built:

`mage build`

Afterwards the service can be started:

`go run .`

### Docker

**Deployment**

The following **active** endpoints exist:

* prod, CDN: https://epr.elastic.co

Additionally, the following **frozen** endpoints exist and are **no longer updated**:

* staging, CDN: https://epr-staging.elastic.co
* snapshot, CDN: https://epr-snapshot.elastic.co/
* experimental, CDN: https://epr-experimental.elastic.co
* 7.9, CDN: https://epr-7-9.elastic.co

**General**
```
docker build .
docker run -p 8080:8080 {image id from prior step}
```

**Commands ready to cut-and-paste**
```
docker build --rm -t docker.elastic.co/package-registry/package-registry:main .
docker run -it -p 8080:8080 $(docker images -q docker.elastic.co/package-registry/package-registry:main)
```

**Listening on HTTPS**
```
docker run -it -p 8443:8443 \
  -v /etc/ssl/package-registry.key:/etc/ssl/package-registry.key:ro \
  -v /etc/ssl/package-registry.crt:/etc/ssl/package-registry.crt:ro \
  -e EPR_ADDRESS=0.0.0.0:8443
  -e EPR_TLS_KEY=/etc/ssl/package-registry.key \
  -e EPR_TLS_CERT=/etc/ssl/package-registry.crt \
  docker.elastic.co/package-registry/package-registry:main
```

#### Docker images published

We publish a Docker image with each successful build commit on branches, tags, or PR.
For each commit we have two docker image tags, one with the commit as tag

`docker.elastic.co/package-registry/package-registry:f999b7a84d977cd19a379f0cec802aa1ef7ca379`

Another Docker tag with the git branch or tag name

* `docker.elastic.co/package-registry/package-registry:main`
* `docker.elastic.co/package-registry/package-registry:pr-111`
* `docker.elastic.co/package-registry/package-registry:v0.2.0`

If you want to run the most recent registry for development, run the main tag.

These images contain only the package registry, they don't contain any package.

### Testing with Kibana

The Docker image of Package Registry is just an empty distribution without any packages.
To test it with Kibana using [elastic-package](https://github.com/elastic/elastic-package),
you need to build a new package-registry docker image first from your required branch.

0. Make sure you've built the Docker image for Package Registry (let's consider in this example `main`):

   ```bash
   docker build --rm -t docker.elastic.co/package-registry/package-registry:main .
   ```

1. Open the Dockerfile used by elastic-package and change the base image for the Packge Registry (use `main` instead of `v1.15.0`):
    - Usually the path would be `${HOME}/.elastic-package/profiles/default/stack/Dockerfile.package-registry`
    - This Dockerfile already enables the Proxy mode (more info at [section](#proxy-mode))

   ```
   FROM docker.elastic.co/package-registry/package-registry:main
   ```

2. Now you're able to start the stack using Elastic Package (Elasticsearch, Kibana, Agent, Fleet Server) with your own Package Registry:

   ```
   elastic-package stack up -v -d
   ```


### Healthcheck

For Docker / Kubernetes the `/health` endpoint can be queried. As soon as `/health` returns a 200, the service is ready.

## Configuration

Package Registry needs to be configured with the source of packages. This
configuration is loaded by default from the `config.yml` file. An example file
is provided with the distribution.

Cache headers can also be configured in the configuration file. They are used
to inform clients about the amount of time a resource is considered fresh. Check
the reference configuration file for the available settings.

Additional runtime settings can be provided using flags, for more information
about the available flags, use `package-registry -help`. Flags can be provided
also as environment variables, in their uppercased form and prefixed by `EPR_`. For example, the
following commands are equivalent:
```
EPR_DRY_RUN=true package-registry
```
```
package-registry -dry-run
```

## Troubleshooting

Package Registry can generate debugging logs when started with the `-log-level` flag. For example

```
EPR_LOG_LEVEL=debug package-registry
```

```
package-registry -log-level debug
```

Or with Docker

```
docker run -it -e "EPR_LOG_LEVEL=debug" <docker-image-identifier>
```

## Performance monitoring

Package Registry is instrumented with the [Elastic APM Go Agent](https://www.elastic.co/guide/en/apm/agent/go/2.x/index.html).
This Agent collects some [system and runtime metrics](https://www.elastic.co/guide/en/apm/agent/go/2.x/metrics.html), and detailed information
about every request handled by the service. You can read more about the kind of
information collected for requests in the [APM Guide](https://www.elastic.co/guide/en/apm/guide/8.2/data-model.html).

You can configure the agent to send the data to any APM Server using the following environment variables:

* `ELASTIC_APM_SERVER_URL`: Address of the APM Server. Instrumentation is
  disabled in Package Registry if this variable is not set.
* `ELASTIC_APM_API_KEY`: API key to use to authenticate with the APM Server, if needed.
* `ELASTIC_APM_SECRET_TOKEN`: If configured in the APM Server, this token has to
  be the same in the agents sending data.
* `ELASTIC_APM_TRANSACTION_SAMPLE_RATE`: Sample rate for transaction collection,
  it can be a value from 0.0 to 1.0. 1.0 is the default value, that collects all
  transactions.

You can find a full reference of these and other options in the Elastic APM Go
Agent [configuration guide](https://www.elastic.co/guide/en/apm/agent/go/current/configuration.html).

## Performance profiling

You can enable the HTTP profiler in Package Registry starting it with the `-httpprof <address>` flag.
It will be listening in the given address.

You can read more about this profiler and the available endpoints in the [pprof documentation](https://pkg.go.dev/net/http/pprof).

## Metrics

Package registry can be instrumented to expose Prometheus metrics under `/metrics` endpoint.
By default this endpoint is disabled.

To enable this instrumentation, the required address (host and port) where this endpoint needs
to run must be set using the parameter `metrics-address` (or the `EPR_METRICS_ADDRESS` environment variable).
For example:

```
package-registry --metrics-address 0.0.0.0:9000
```

## Proxy Mode

The Docker image of Package Registry is just an empty distribution without any packages.
You can enable in Package Registry the proxy mode. This mode allows to take into account all the packages
from other endpoint as part of the responses.

This mode is enabled with the parameter `-feature-proxy-mode=true` (or `EPR_FEATURE_PROXY_MODE` environment variable).
And it will use by default as proxy endpoint `https://epr.elastic.co`. This endpoint can be customized using the parameter `-proxy-to`
(or `EPR_PROXY_TO`).
For example:

```
package-registry --feature-proxy-mode=true -proxy-to=https://epr.elastic.co
```


## Release

New versions of the package registry need to be released from time to time. The following steps should be followed to create a new release:

1. Create a new branch with the changes to be done for the release
2. Update the changelog by putting in a line for the release, remove all not needed section and put in a new Unreleased section. Don't forget to update the links to the diffs.
3. Update the registry version in the `main.go` file to be the same version as the release is planned and update the generated files with `go test . -generate`.
4. Open a pull request and get it merged
5. Tag the new release by creating a new release in Github, put in the changelog in the release
6. Update the main.go to increase the version number to the version of the potential next release version.

CI automatically creates a new Docker image which will be available under `docker.elastic.co/package-registry/package-registry:vA.B.C` a few minutes after creating the tag.

After the new registry Docker image is available, update the following projects that consume it:
- Integrations: Update the version of the Package Registry Docker image as shown in this [sample PR](https://github.com/elastic/integrations/pull/581).
- Elastic Package: Update the version of the Package Registry used in the docker-compose as shown in this [sample PR](https://github.com/elastic/elastic-package/pull/1254)
