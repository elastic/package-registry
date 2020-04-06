# Contributing

This page is intended for contributors to the registry and packages.

## Definitions

### Package

A Package contains the dashboards, visualisations, and configurations to monitor the logs and metrics of a particular technology or group of related services, such as “MySQL”, or “System”.

A Package consists of:

* Name
* Zero or more dashboards and visualisations and Canvas workpads
* Zero or more ML job definitions
* Zero or more dataset templates

The package is versioned.

### Integration

An integration is a specific type of a _package_ defining datasets used to observe the same product (logs and metrics).

### Dataset Template

The dataset template is part of a package and contains all the assets which are needed to create a dataset. Example for assets are: ingest pipeline, agent config template, ES index template, ...

The dataset templates are inside the package directory under `dataset`.

A dataset template consists of:

* An alias templates (or the fields.yml to create it)
* Zero or more ingest pipelines
* An Elastic Agent config template

### Migration from Beats

A defined importing procedure used to transform both, Filebeat and Metricbeat modules related to
the same observed product, into a single integration. The integration contains extracted dataset configuration of beat
modules, hence no module are required to exist anymore.

## Package structure

### Elements

Link: https://github.com/elastic/package-registry/blob/master/ASSETS.md

### Reference packages

The following packages can be considered as reference points for all integrations.

#### Integration: reference

Link: https://github.com/elastic/package-registry/tree/master/dev/packages/example/reference-1.0.0

The directory contains mandatory manifest files defining the integration and its datasets. All manifests have fields
annotated with comments to better understand their goals.

Keep in mind that this package doesn't contain all file resources (images, screenshots, icons) referenced in manifests.
Let's assume that they're also there.

#### Integration: mysql

Link: https://github.com/mtojek/package-registry/tree/package-mysql-0.0.2/dev/packages/alpha/mysql-0.0.2

The MySQL integration was the first integration built using the [https://github.com/elastic/package-registry/tree/master/dev/import-beats][import-beats] script.
The script imported filesets and metricsets from both MySQL modules, and converted them to a package.

The MySQL integration contains all parts that should be present (or are required) in the integration package.

After using the _import-beats_ script, the integration has been manually adjusted and extended with dedicated docs.

## Create a new integration

This section describes steps required to perform to build a new integration. If you plan to prepare the integration
with a product unsupported by [https://github.com/elastic/beats][Beats], feel free to skip the section about importing
existing modules.

### Import from existing modules

The import procedure heavily uses on the _import-beats_ script. If you are interested how does it work internally,
feel free to review the script's [https://github.com/elastic/package-registry/blob/master/dev/import-beats/README.md][README].

1. Focus on the particular product (e.g. MySQL, ActiveMQ) you would like to integrate with.
2. Prepare the developer environment:
    1. Clone/refresh the following repositories:
        * https://github.com/elastic/beats
        * https://github.com/elastic/ecs
        * https://github.com/elastic/eui
        * https://github.com/elastic/kibana
        
       Make sure you don't have any manual changes applied as they will reflect on the integration.
    2. Clone/refresh the Elastic Package Registry (EPR) to always use the latest version of the script:
        * https://github.com/elastic/package-registry
    3. Make sure you've the `mage` tool installed.
3. Boot up required dependencies:
    1. Elasticseach instance:
        * Kibana's dependency
    2. Kibana instance:
        * used to migrate dashboards, if not available, you can skip the generation (`SKIP_KIBANA=true`)

    _Hint_. There is dockerized environment in beats (`cd testing/environments`). Boot it up with the following command:
    `docker-compose -f snapshot.yml -f local.yml up --force-recreate elasticsearch kibana`.
4. Create a new branch for the integration in the EPR project (diverge from master).
5. Run the command: `mage import-beats` to start the import process.
    
    The result of running the `import-beats` script are refreshed and updated integrations.

    It will take a while to finish, but the console output should be updated frequently to track the progress.
    The command must end up with the exit code 0. Kindly please to open an issue if it doesn't.
    
    Generated packages are stored by default in the `dev/packages/beats` directory. Generally, the import process
    updates all of the integrations, so don't be surprised if you notice updates to multiple integrations, including
    the one you're currently working on (e.g. `dev/packages/beats/foobarbaz-0.0.1`). You can either commit this changes or
    leave them for later.
    
6. Copy the package output for your integration (e.g. `dev/packages/beats/foobarbaz-0.0.1`) to the _alpha_ directory and
    raise the version manually: `dev/packages/alpha/foobarbaz-0.0.2`.

### Fine-tune the integration

#### Motivation

TODO

#### Checklist

## Testing and validation

TODO


[import-beats]: https://github.com/elastic/package-registry/tree/master/dev/import-beats


[Beats]: https://github.com/elastic/beats
