# import-beats

The script is responsible for importing existing beats modules and transforming
them into integration packages compatible with Elastic Package Registry (EPR).

The `import-beats` script depends on active Kibana instance, which is used to
migrate existing dashboards to a newer version.

## Usage

```bash
$ go run dev/import-beats/*.go -help
  Usage of /var/folders/gz/dht4sjdx5w9f72knybys10zw0000gn/T/go-build249388773/b001/exe/agent:
    -beatsDir string
       Path to the beats repository (default "../beats")
    -ecsDir string
       Path to the Elastic Common Schema repository (default "../ecs")
    -euiDir string
       Path to the Elastic UI framework repository (default "../eui")
    -kibanaDir string
       Path to the kibana repository (default "../kibana")
    -kibanaHostPort string
       Kibana host and port (default "http://localhost:5601")
    -outputDir string
       Path to the output directory (default "dev/packages/beats")
    -skipKibana
       Skip storing Kibana objects
``

## Import all packages

1. Make sure that the following repositories have been fetched locally:
https://github.com/elastic/beats
https://github.com/elastic/ecs
https://github.com/elastic/eui
https://github.com/elastic/kibana
2. Start Kibana server (make sure the endpoint is accessible: http://localhost:5601/)
2. Run the importing procedure with the following command:

```bash
$ mage ImportBeats
```

## Troubleshooting

*Importing process takes too long.*

While developeing, you can try to perform the migration with skipping migration of all Kibana objects,
as this is the most time consuming part of whole process:

```bash
$ SKIP_KIBANA=true mage ImportBeats
```
