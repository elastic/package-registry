# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).


## [Unreleased](https://github.com/elastic/package-registry/compare/v0.3.0...master)

### Breaking changes

* Change package path from /package/{packagename}-{version} to /package/{packagename}/{version} [#](https://github.com/elastic/integrations-registry/pull/)

### Bugfixes

* Remove caching headers in case of errors. [#275](https://github.com/elastic/integrations-registry/pull/275)

### Added

* Allow to set cache times through config. [#271](https://github.com/elastic/integrations-registry/pull/271)
* Make README.md file a required file for a package. [#287](https://github.com/elastic/integrations-registry/pull/287)
*  Add stream fields to each dataset [#296](https://github.com/elastic/integrations-registry/pull/296)

### Deprecated

### Known Issue



## [Unreleased](https://github.com/elastic/package-registry/compare/v0.2.0...v0.3.0)

### Breaking changes

* Change `requirements.kibana.version.min/max` to `requirements.kibana.versions: {semver-range}`
* Encode Kibana objects during packaging. [#157](https://github.com/elastic/integrations-registry/pull/157)
* Prefix package download url with `/epr/{package-name}`.
* Remove dataset.name but introduce dataset.id and dataset.path. [#176](https://github.com/elastic/package-registry/pull/176)

### Bugfixes

* Fix header for `tar.gz` files from `application/json` to `application/gzip`. [#154](https://github.com/elastic/integrations-registry/pull/154)

### Added

* Add `/health` and `/health?ready=1` endpoint for healthcheck. [#151](https://github.com/elastic/integrations-registry/pull/151)
* Add `default` config to dataset manifest. [#148](https://github.com/elastic/integrations-registry/pull/148)
* Update Golang version to 1.13.4. [#159](https://github.com/elastic/integrations-registry/pull/159)
* Add missing assets from datasets. [#146](https://github.com/elastic/integrations-registry/pull/146)
* Add `format_version` to define the package format.
* Add dataset array to package info endpoint. [#162](https://github.com/elastic/integrations-registry/pull/162)
* Add path field to search and package info endpoint. [#174](https://github.com/elastic/integrations-registry/pull/174)
* Add download field to package info endpoint. [#174](https://github.com/elastic/integrations-registry/pull/174)
* Add `package` field to dataset. [#189](https://github.com/elastic/integrations-registry/pull/189)
* Add support for datasources. [#216](https://github.com/elastic/integrations-registry/pull/216) [#212](https://github.com/elastic/integrations-registry/pull/212)


## [0.2.0](https://github.com/elastic/package-registry/compare/v0.1.0...v0.2.0)

### Breaking changes

* Package Kibana compatiblity version is changed to `"kibana": { "max": "1.2.3"}` [#134](https://github.com/elastic/integrations-registry/pull/134)
* Rename `integrations-registry` to `package-registry`. [#138](https://github.com/elastic/integrations-registry/pull/138)
* Remove `packages.path` config and replace it with `public_dir` config. [#118](https://github.com/elastic/integrations-registry/pull/118)

### Bugfixes

* Change empty /search API output from `null` to `[]`. [#111](https://github.com/elastic/integrations-registry/pull/111)

### Added

* Add validation check that Kibana min/max are valid semver versions. [#99](https://github.com/elastic/integrations-registry/pull/99)
* Adding Cache-Control max-age headers to all http responses set to 1h. [#101](https://github.com/elastic/integrations-registry/pull/101)
* Validate packages to guarantee only predefined categories can be used. [#100](https://github.com/elastic/integrations-registry/pull/100)
* Cache all manifest on service startup for resource optimisation. [#103](https://github.com/elastic/integrations-registry/pull/103)
* Fix Docker image to specific Golang version. [#107](https://github.com/elastic/integrations-registry/pull/107)
* Add .dockerignore file for slimmer image. [#104](https://github.com/elastic/integrations-registry/pull/104)
* Move package generation to its own package. [#112](https://github.com/elastic/integrations-registry/pull/112)
* Remove not needed files in Docker image. [#106](https://github.com/elastic/integrations-registry/pull/106)
* Add healthcheck to docker file. [#115](https://github.com/elastic/integrations-registry/pull/115)
* Make caching headers configurable per endpoint. [#116](https://github.com/elastic/integrations-registry/pull/116)
* Add readme entry to package endpoint. [#128](https://github.com/elastic/integrations-registry/pull/128)


## [0.1.0]

First tagged release. No changelog existed so far.
