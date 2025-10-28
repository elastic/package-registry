# Elastic Package Registry (EPR)

## API

Endpoints:

* `/`: Info about the registry
* `/health`: Health of the service. Returns 200 if service is ready.
* `/search`: Search for packages. By default returns all the most recent packages available.
* `/categories`: List of the existing package categories and how many packages are in each category.
* `/package/{name}/{version}`: Info about a package
* `/epr/{name}/{name}-{version}.zip`: Download a package

### /search

The `/search` API endpoint has few additional query parameters. More might be added in the future, but for now these are:

* `kibana.version`: Filters out all the packages which are not compatible with the given Kibana version. If it is set to `7.3.1` and
  a package requires 7.4, the package will not be returned or an older compatible package will be shown.
  By default this endpoint always returns only the newest compatible package.
* `category`: Filters the package by the given category. Available categories can be seen when going to `/categories` endpoint.
* `package`: Filters by a specific package name, for example `mysql`. Returns the most recent version.
* `all`: This can be set to `true` to list all package versions. This is set to `false` by default.
* `type`: Filters by a specific package type, for example `input`.
* `capabilities`: Filters the packages according to the given capabilities. This query parameter accepts a comma-separated list.
* `spec.min` and `spec.max`: Filters the packages by their `format_version` field given the version (major and minor numbers) set in the query parameter. It is not required to set both parameters. Examples:
    - `?spec.min=2.2&spec.max=3.3`
    - `?spec.max=3.3`
    - `?spec.min=3.0`
* `discovery`: Returns the packages that define the `discovery` setting and fulfill the conditions in the query parameter. There are two different options:
    - Based on fields: Packages must include discovery fields in their manifest and all of fields must be included in the list included in the request
        - Example: Packages that contain both `process.pid` and `host.os.name` fields: `?discovery=fields:process.pid,host.os.name`
    - Based on datasets: Packages must include discovery datasets in their manifest and at least one of the datasets must be included in the list included in the request:
        - Example: Packages that contain at least one of `nginx.access` or `nginx.error` datasets: `?discovery=datasets:nginx.access,nginx.error`
* `prerelease`: This can be set to `true` to list prerelease versions of packages. Versions are considered prereleases if they are not stable according to semantic versioning, that is, if they are 0.x versions, or if they contain a prerelease tag. This is set to `false` by default.
* `experimental` (deprecated): This can be set to `true` to list packages considered to be experimental. This is set to `false` by default.

The different query parameters above can be combined, so `?package=mysql&kibana.version=7.3.0` will return all mysql package versions
which are compatible with `7.3.0`.

### /categories

The `/categories` API endpoint has two additional query parameters.

* `prerelease`: This can be set to `true` to list prerelease versions of packages. Versions are considered prereleases if they are not stable according to semantic versioning, that is, if they are 0.x versions, or if they contain a prerelease tag. This is set to `false` by default.
* `experimental` (deprecated): This can be set to `true` to list categories from experimental packages. This is set to `false` by default.
* `include_policy_templates`: This can be set to `true` to include categories from policy templates. This is set to `false` by default.
* `capabilities`: List categories filtering the packages according to the given capabilities. This query parameter accepts a comma-separated list.
* `spec.min` and `spec.max`: List categories filtering the packages by their `format_version` field given the version (major and minor numbers) set in the query parameter. It is not required to set both parameters. Examples:
    - `?spec.min=2.2&spec.max=3.3`
    - `?spec.max=3.3`
    - `?spec.min=3.0`
* `discovery`: List categories filtering the packages that define the `discovery` setting and fulfill the conditions in the query parameter. These query parameter follow the same syntax and behaviour to obtain the corresponding categories as in [`/search` endpoint](#search).

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

## Directories

* testdata/package: Contains the packages for testing.

## Running

There are several options to run this for development purposes.

### Go command

We recommend using [GVM](https://github.com/andrewkroh/gvm), same as done in the CI.
This tool allows you to install multiple versions of Go, setting the Go environment in consequence: `eval "$(gvm 1.25.1)"`

To use the correct golang version, run:

```
gvm use $(cat .go-version)
```

Afterwards the service can be started for development with:

```
go run .
```

### Single binary

You can build the `package-registry` binary using [`mage`](https://magefile.org):
```
mage build
```

Once built, the Package Registry can be run as `./package-registry`. Find below
more details about the available configuration options.


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
```bash
mage dockerBuild main
docker run --rm -it -p 8080:8080 docker.elastic.co/package-registry/package-registry:main
```

**Testing service with local packages**
- Default configuration used in the image: [`config.docker.yml`](./config.docker.yml):

```bash
docker run --rm -it -p 8080:8080 \
  -v /path/to/local/packages:/packages/package-registry \
  docker.elastic.co/package-registry/package-registry:main
```

**Listening on HTTPS**
```bash
docker run --rm -it -p 8443:8443 \
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
You can test your own code with Kibana using [elastic-package](https://github.com/elastic/elastic-package).
For that, you need to build a new Package Registry docker image from your required branch:

0. Make sure you've built the Docker image for Package Registry (let's consider in this example `main`):

   ```bash
   mage dockerBuild main
   ```

1. Build `elastic-package` changing the base image used for the Package Registry docker image (use `main` instead of `v1.24.0`):
    - Update the docker image (and docker tag) used for package-registry [here](https://github.com/elastic/elastic-package/blob/db40e519788a4340f21d166e012f7a2298633cc4/internal/stack/versions.go#L9).
        - Dockerfile used in `elastic-package` already [enables the Proxy mode](https://github.com/elastic/elastic-package/blob/db40e519788a4340f21d166e012f7a2298633cc4/internal/stack/_static/Dockerfile.package-registry.tmpl#L9) (more info at [section](#proxy-mode)).

      ```golang
      PackageRegistryBaseImage = "docker.elastic.co/package-registry/package-registry:main"
      ```
    - Build `elastic-package` (follow [elastic-package instructions](https://github.com/elastic/elastic-package/blob/main/README.md#development)).

2. Now you're able to start the stack using Elastic Package (running Elasticsearch, Kibana, Agent and Fleet Server services) with your own Package Registry service:
   ```shell
   elastic-package stack up -v -d
   ```

### Healthcheck

Availability of the service can be queried using the `/health` endpoint. As soon as `/health` returns a 200, the service is ready to handle requests.

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
```bash
EPR_DRY_RUN=true package-registry
```
```bash
package-registry -dry-run
```

## Troubleshooting

Package Registry can generate debugging logs when started with the `-log-level` flag. For example

```bash
EPR_LOG_LEVEL=debug package-registry
```

```bash
package-registry -log-level debug
```

Or with Docker

```bash
docker run --rm -it -e "EPR_LOG_LEVEL=debug" <docker-image-identifier>
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

```bash
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

```bash
package-registry --feature-proxy-mode=true -proxy-to=https://epr.elastic.co
```

### Storage indexers

Elastic Package Registry (EPR) supports multiple ways to retrieve package information. By default, it uses the File system indexer to read packages (folders or zip files) from the paths defined in the `config.yml`.

[Here](./docs/storage_indexers.md#storage-indexers-used-in-elastic-package-registry) you can read more about the different
indexers available in EPR as well as how to test EPR locally using these storage indexers without configuring a remote bucket.


## Release

New versions of the package registry need to be released from time to time. The following steps should be followed to create a new release:

1. Create a new branch with the changes to be done for the release
2. Update the changelog by putting in a line for the release, remove all not needed section and put in a new Unreleased section. Don't forget to update the links to the diffs.
3. Update the registry version in the `main.go` file to be the same version as the release is planned and update the generated files with `go test . -generate`.
4. Open a pull request and get it merged
5. Tag the new release by creating a new release in GitHub, put in the changelog in the release
6. Update the main.go to increase the version number to the version of the potential next release version.

CI automatically creates a new Docker image which will be available under `docker.elastic.co/package-registry/package-registry:vA.B.C` a few minutes after creating the tag.

After the new registry Docker image is available, update the following projects that consume it:
- Integrations: Update the version of the Package Registry Docker image as shown in this [sample PR](https://github.com/elastic/integrations/pull/581).
- Elastic Package: Update the version of the Package Registry used in the docker-compose as shown in this [sample PR](https://github.com/elastic/elastic-package/pull/1254)

## Development

### Running Tests and Generating JSON Responses

To run tests and generate JSON responses follow these steps:

1. **Main package tests**: Execute the following command to run all tests from the main package and generate its response files:
  ```bash
  mage testGenerateMain
  ```

  or

  ```bash
  go test . -v -generate
  ```


2. **Packages tests**: Use the following command to generate JSON responses from the packages package tests:
  ```bash
  mage testGeneratePackages
  ```

  or

  ```bash
  go test ./packages/... -v -generate
  ```