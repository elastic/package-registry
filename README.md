# EXPERIMENTAL: This is only for experimental use

# Integrations registry

## API

Endpoints:

* `/`: Info about the registry
* `/list`: Lis of all available integration packages
* `/package/{name}`: Info about a package
* `/package/{name}/get`: Download a package

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