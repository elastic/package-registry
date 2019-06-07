# EXPERIMENTAL: This is only for experimental use

# Integrations registry

## API

Endpoints:

* `/`: Info about the registry
* `/list`: Lis of all available integration packages
* `/package/{name}`: Info about a package
* `/package/{name}/get`: Download a package

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

The directory name of a package must be as following: `{integration-name}-{version}`. This makes it possible to store multiple versions of the same packages in one directory and already indicates the version before reading the manifest file. The zipped packaged has the name convention with the name but added `.zip` at the end.

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

## Directories

* packages: Contains all the integrations packages. These are just example integration packages used for development.

## Running

There are two options to run this. Either the service can be run as a go command or inside a docker container.

Go command: `go run main.go`

Docker:

```
docker build .
docker run -p 8080:8080 {container-id}
```
