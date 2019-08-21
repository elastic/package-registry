# EXPERIMENTAL: This is only for experimental use

# Integrations registry

## API

Endpoints:

* `/`: Info about the registry
* `/list`: Lis of all available integration packages
* `/package/{name}`: Info about a package
* `/package/{name}.tar.gz`: Download a package

Examples for each API endpoint can be found here: https://github.com/elastic/integrations-registry/tree/master/docs/api

## Package structure

The structure of each integration package is standardised. It looks as following:

Files to be loaded into the Elastic Stack:

```
{service}/{type}/{filename}
```

Service in the above can be `elasticsearch`, `kibana` or any other component in the Elastic Stack. The type is specific to each service. In the case of Elasticsearch it can be `ingest-pipline`, `index-template` or could also be `index` data. For Kibana it could be `dashboard`, `visualization` or any other saved object type or other types. The names are taken from the API endpoints in each service. The file name needs to be unique inside the directory and best has a descriptive nature or unique id.

Each package can contain 2 additional directories:

* `docs`: Containing documentation files
* `img`: Contains images for the integrations.

On the top level each package contains a `manifest.yml` which describes the package and contains meta information about the package. A basic manifest file looks as following:

```
name: envoyproxy
description: This is the envoyproxy integration.
version: 0.0.2
```

The directory name of a package must be as following: `{integration-name}-{version}`. This makes it possible to store multiple versions of the same packages in one directory and already indicates the version before reading the manifest file. The tar packaged has the name convention with the name but added `.tar.gz` at the end.

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

More details about each asset can be found in ASSETS.md

## Directories

* build/packages: Contains all the example integrations packages. These are only example integration packages used for development. Run `mage build` to generate these.
* dev: The dev packages contains at the moment a template to build example integrations in automated way.
* testdata/package: Contains the package for testing. This also serves as an example for a package.

## Running

There are several options to run this.

### Go command

When using the go command, first the example packages must be built:

`mage build`

Afterwards the service can be started:

`go run .`

### Docker
**Example**
An example registry is running on `http://integrations-registry.app.elstc.co/`. This is updated from time to time to be in sync with master.

**General**
```
docker build .
docker run -p 8080:8080 {image id from prior step}
```

**Commands ready to cut-and-paste**
```
docker build --rm -t integrations_registry:latest .
docker run -i -t -p 8080:8080 $(docker images -q integrations_registry:latest)
```

## Generated Example packages

For easier testing of the integrations manager, the registry allows to generate some example packages. The packages
to generate are taken out of `dev/packages.yml`. These values are then used to build a package based on the template
files inside `dev/package-template` and values are replaced. In addition it's possible to add icons for each package 
under icons. The name of the icon must match the package name.

The command to generate the packages is:

```
mage generatePackages
```
