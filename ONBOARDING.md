# Onboarding

_This page describes the onboarding process for contributors working on integrations development or migrating existing
Beats modules to the repository._

## Glossary

*Package* - a basic distribution form provided by the Elastic Package Registry (EPR). Currently, there is only one type
available - _integration_. The package is versioned, contains manifests, artifacts, documentation, fields definitions,
etc.

*Integration* - a specific _package_ type defining datasets used to observe the same product (logs and metrics).

*Migration from Beats* - a defined importing procedure used to transform both, Filebeat and Metricbeat modules related to
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

## Creating new integration

TODO

### Import from existing modules

Link: https://github.com/elastic/package-registry/blob/master/dev/import-beats/README.md

TODO

### Fine-tuning the integration - checklist

TODO

## Testing and validation

TODO
