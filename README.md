# Elastic Package Registry (EPR)

## API

Endpoints:

* `/`: Info about the registry
* `/search`: Search for packages. By default returns all the most recent packages available.
* `/categories`: List of the existing package categories and how many packages are in each category.
* `/package/{name}/{version}`: Info about a package
* `/epr/{name}/{name}-{version}.tar.gz`: Download a package

Examples for each API endpoint can be found here: https://github.com/elastic/package-registry/tree/master/docs/api

The `/search` API endpoint has few additional query parameters. More might be added in the future, but for now these are:

* kibana: Filters out all the packages which are not compatible with the given Kibana version. If it is set to `7.3.1` and
  a package requires 7.4, the package will not be returned or an older compatible package will be shown.
  By default this endpoint always returns only the newest compatible package.
* category: Filters the package by the given category. Available categories can be seend when going to `/categories` endpoint.
* package: Filters by a specific package name, for example `mysql`. In contrast to the other endpoints, it will return
  by default all versions of this package.
* internal: This can be set to true, to also list internal packages. This is set to `false` by default.

The different query parameters above can be combined, so `?package=mysql&kibana=7.3.0` will return all mysql package versions
which are compatible with `7.3.0`.

## Package structure

The structure of each package is standardised. It looks as following:

Files to be loaded into the Elastic Stack:

```
{service}/{type}/{filename}
```

Service in the above can be `elasticsearch`, `kibana` or any other component in the Elastic Stack. The type is specific to each service. In the case of Elasticsearch it can be `ingest_pipeline`, `index_template` or could also be `index` data. For Kibana it could be `dashboard`, `visualization` or any other saved object type or other types. The names are taken from the API endpoints in each service. The file name needs to be unique inside the directory and best has a descriptive nature or unique id.

Each package can contain 2 additional directories:

* `docs`: Containing documentation files
* `img`: Contains images for the package.

On the top level each package contains a `manifest.yml` which describes the package and contains meta information about the package. A basic manifest file looks as following:

```
name: envoyproxy
description: This is the envoyproxy package.
version: 0.0.2
```

The directory name of a package must be as following: `{package-name}-{version}`. This makes it possible to store multiple versions of the same packages in one directory and already indicates the version before reading the manifest file. The tar packaged has the name convention with the name but added `.tar.gz` at the end.

A full example with the directory structure looks as following:

```
├── docs
│   └── docs.asciidoc
├── elasticsearch
│   └── ingest-pipeline
│       ├── pipeline-entry.json
│       ├── pipeline-http.json
│       ├── pipeline-json.json
│       ├── pipeline-plaintext.json
│       └── pipeline-tcp.json
├── img
│   └── kibana-envoyproxy.jpg
├── kibana
│   ├── dashboard
│   │   └── 0c610510-5cbd-11e9-8477-077ec9664dbd.json
│   ├── index-pattern
│   │   └── filebeat-*.json
│   ├── search
│   └── visualization
│       ├── 0a994af0-5c9d-11e9-8477-077ec9664dbd.json
│       ├── 36f872a0-5c03-11e9-85b4-19d0072eb4f2.json
└── manifest.yml
```

More details about each asset can be found in the reference package: /testdata/package/reference

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
* dev: The dev packages contains at the moment a template to build example packages in automated way.
* testdata/package: Contains the package for testing. This also serves as an example for a package.

## Running

There are several options to run this.

### Go command

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

The following endpoints exist:

* prod, no CDN: https://epr.ea-web.elastic.dev
* prod, CDN: https://epr.elastic.co
* staging, no CDN: https://epr-staging.ea-web.elastic.dev
* staging, CDN: https://epr-staging.elastic.co
* experimental, no CDN: https://epr-experimental.ea-web.elastic.dev/
* experimental, CDN: https://epr-experimental.elastic.co/

An dev registry is running on `https://epr-staging.elastic.co/`. This is updated from time to time to be in sync with master.

The deployment runs on an Elastic internal k8s cluster. To get all the deployments for the registry use the following command:

```
kubectl get deployment -n package-registry
```

This will output the list of available deployments. To do a rolling restart of the staging deployment run:

```
kubectl rollout restart deployment package-registry-staging-vanilla -n package-registry
```

**General**
```
docker build .
docker run -p 8080:8080 {image id from prior step}
```

**Commands ready to cut-and-paste**
```
docker build --rm -t docker.elastic.co/package-registry/package-registry:master .
docker run -i -t -p 8080:8080 $(docker images -q docker.elastic.co/package-registry/package-registry:master)
```

#### Docker images published

We publish a Docker image with each successful build commit on branches, tags, or PR.
For each commit we have two docker image tags, one with the commit as tag

`docker.elastic.co/package-registry/package-registry:f999b7a84d977cd19a379f0cec802aa1ef7ca379`

Another Docker tag with the git branch or tag name

* `docker.elastic.co/package-registry/package-registry:master`
* `docker.elastic.co/package-registry/package-registry:pr-111`
* `docker.elastic.co/package-registry/package-registry:v0.2.0`

If you want to run the most recent registry, run the master tag.

### Healthcheck

For Docker / Kubernetes the `/health` endpoint can be queried. As soon as `/health` returns a 200, the service is ready.

## Release

New versions of the package registry need to be released from time to time. The following steps should be followed to create a new release:

1. Create a new branch with the changes to be done for the release
2. Update the changelog by putting in a line for the release, remove all not needed section and put in a new Unreleased section. Don't forget to update the links to the diffs.
3. Update the registry version in the `main.go` file to be the same version as the release is planned and update the generated files with `go test . -generate`.
4. Open a pull request and get it merged
5. Tag the new release by creating a new release in Github, put in the changelog in the release
6. Update the main.go to increase the version number to the version of the potential next release version.

CI automatically creates a new Docker image which will be available under `docker.elastic.co/package-registry/package-registry:vA.B.C` a few minutes after creating the tag.

As a new registry is normally released to bring new features to the package-storage, follow the docs in the [Package Storage](https://github.com/elastic/package-storage#update-package-registry-for-a-distribution) repository on how to update the distributions.
