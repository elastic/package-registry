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



### Dataset Template

The dataset template is part of a package and contains all the assets which are needed to create a dataset. Example for assets are: ingest pipeline, agent config template, ES index template, ...

The dataset templates are inside the package directory under `dataset`.

A dataset template consists of:

* An alias templates (or the fields.yml to create it)
* Zero or more ingest pipelines
* An Elastic Agent config template

## How to build a package

Coming soon ...
