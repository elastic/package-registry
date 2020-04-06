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

An integration is a specific _package_ type defining datasets used to observe the same product (logs and metrics).

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
annotated with comments to better understanding their goals.

Keep in mind that this package doesn't contain all file resources (images, screenshots, icons) referenced in manifests.
Let's assume that they're also there.

#### Integration: mysql

Link: https://github.com/mtojek/package-registry/tree/package-mysql-0.0.2/dev/packages/alpha/mysql-0.0.2

TODO

## Create a new integration

TODO

### Import from existing modules

Link: https://github.com/elastic/package-registry/blob/master/dev/import-beats/README.md

TODO

### Fine-tune the integration - checklist

TODO

## Testing and validation

TODO
