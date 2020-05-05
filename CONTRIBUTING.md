# Contributing

This page is intended for contributors to the registry and packages.

## Definitions

### Package

A package contains the dashboards, visualisations, and configurations to monitor the logs and metrics of a particular technology or group of related services, such as “MySQL”, or “System”.

The package consists of:

* Name
* Zero or more dashboards and visualisations and Canvas workpads
* Zero or more ML job definitions
* Zero or more dataset templates

The package is versioned.

### Integration

An integration is a specific type of a _package_ defining datasets used to observe the same product (logs and metrics).

### Dataset Template

A dataset template is part of a package and contains all the assets which are needed to create a dataset. Example for assets are: ingest pipeline, agent config template, ES index template, ...

Dataset templates are inside the package directory under `dataset`.

The dataset template consists of:

* An alias templates (or the fields.yml to create it)
* Zero or more ingest pipelines
* An Elastic Agent config template

### Migration from Beats

A defined importing procedure used to transform both Filebeat and Metricbeat modules, related to
the same observed product, into a single integration. The integration contains extracted dataset configuration of beat
modules, hence no modules are required to exist anymore.

The migration procedure is described in the Integrations CONTRIBUTING guide: https://github.com/elastic/integrations/blob/master/CONTRIBUTING.md

