# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased](https://github.com/elastic/package-registry/compare/v0.17.0...master)

### Breaking changes

### Bugfixes

### Added

### Deprecated

### Known Issues

## [0.17.0](https://github.com/elastic/package-registry/compare/v0.16.0...0.17.0)

### Breaking changes

### Bugfixes

* Fix the package not loading if it has an accidental file left in the package root directory. Add semver validation of the version directory. [#673] (https://github.com/elastic/package-registry/pull/673)

### Added

* Add "dataset_is_prefix" field to data stream. [#674] (https://github.com/elastic/package-registry/pull/674)

### Deprecated

### Known Issues

## [0.16.0](https://github.com/elastic/package-registry/compare/v0.15.0...v0.16.0)

### Breaking changes

### Bugfixes

### Added

* Package validation can be disabled via command line option. [#667] (https://github.com/elastic/package-registry/pull/667)

### Deprecated

### Known Issues

## [0.15.0](https://github.com/elastic/package-registry/compare/v0.14.0...v0.15.0)

### Breaking changes

### Bugfixes

### Added

* Add "hidden" field to data stream. [#660] (https://github.com/elastic/package-registry/pull/660)
* Add "ilm_policy" field to data stream. [#657] (https://github.com/elastic/package-registry/pull/657)

### Deprecated

### Known Issue

## [0.14.0](https://github.com/elastic/package-registry/compare/v0.13.0...v0.14.0)

### Breaking changes

### Bugfixes

* Set cache headers for 404 for all API endpoints to private, no-store.[#652](https://github.com/elastic/package-registry/pull/652)

### Added

* Add "traces" as legal event type. [#656](https://github.com/elastic/package-registry/pull/656)
* Add input-level `template_path` field. [#655](https://github.com/elastic/package-registry/pull/655)

### Deprecated

### Known Issue

## [0.13.0](https://github.com/elastic/package-registry/compare/v0.12.1...v0.13.0)

### Breaking changes

### Bugfixes
* Set cache headers for 404 and 400 to 0. [#649](https://github.com/elastic/package-registry/pull/649)

### Added

### Deprecated

### Known Issue

## [v0.12.1](https://github.com/elastic/package-registry/compare/v0.12.0...v0.12.1)

### Breaking changes

### Bugfixes

* Expose proper EPR version. [#644](https://github.com/elastic/package-registry/pull/644)

### Added

### Deprecated

### Known Issue

## [v0.12.0](https://github.com/elastic/package-registry/compare/v0.11.0...v0.12.0)

### Breaking changes

* Rename config template to policy template and dataset to data stream. [#641](https://github.com/elastic/package-registry/pull/641)

### Bugfixes

### Added

* Add validation for icons and screenshots. [#537](https://github.com/elastic/package-registry/pull/537)

### Deprecated

### Known Issue

## [v0.11.0](https://github.com/elastic/package-registry/compare/v0.10.0...v0.11.0)

### Breaking changes

* Rename version to service.version in index handler. [#633](https://github.com/elastic/package-registry/pull/633)
* Remove config `public_dir` which is replaced by `package_paths`. [#632](https://github.com/elastic/package-registry/pull/632)
* Ship packages as zip instead of tar.gz [#628](https://github.com/elastic/package-registry/pull/628)
* Rename image src to path and have src as the original value from the manifest. [#629](https://github.com/elastic/package-registry/pull/629)

### Added

* Add `cache_time.index` as config option. [#631](https://github.com/elastic/package-registry/pull/631)

## [v0.10.0](https://github.com/elastic/package-registry/compare/v0.9.0...v0.10.0)

### Breaking changes

* Change dataset.* fields to data_stream.* fields. [#622](https://github.com/elastic/package-registry/pull/622)

## [v0.9.0](https://github.com/elastic/package-registry/compare/v0.8.0...v0.9.0)

### Breaking changes

* Change dataset.* fields to datastream.* fields. [#618](https://github.com/elastic/package-registry/pull/618)

## [v0.8.0](https://github.com/elastic/package-registry/compare/v0.7.1...v0.8.0)

### Breaking changes

### Bugfixes

### Added

* Allow numbers in package names. [#614](https://github.com/elastic/package-registry/pull/614)

### Deprecated

### Known Issue

## [v0.7.1](https://github.com/elastic/package-registry/compare/v0.7.0...v0.7.1)

### Bugfixes

* Always populate template_path. [#600](https://github.com/elastic/package-registry/pull/600)

## [v0.7.0](https://github.com/elastic/package-registry/compare/v0.6.0...v0.7.0)

### Bugfixes

* Fix Gogle Cloud tag typo. [#592](https://github.com/elastic/package-registry/pull/592)

### Added

* Add missing MIME types. [#599](https://github.com/elastic/package-registry/pull/599)
* Make `release` field available as part of `/search` endpoint. [#591](https://github.com/elastic/package-registry/pull/591)

### Deprecated

* Remove development mode. [#597](https://github.com/elastic/package-registry/pull/597)

## [v0.6.0](https://github.com/elastic/package-registry/compare/v0.4.0...v0.6.0)

### Breaking changes

* Change stream.* fields to dataset.* fields. [#492](https://github.com/elastic/package-registry/pull/492)
* Remove `solution` entry support in package manfiest. [#504](https://github.com/elastic/package-registry/pull/504)
* Remove support for Elasticsearch requirements [#516](https://github.com/elastic/package-registry/pull/516)
* Rename `kibana` query param to `kibana.version`. [#518](https://github.com/elastic/package-registry/pull/518)
* Remove `removable` flag from package manifest. [#532](https://github.com/elastic/package-registry/pull/532)
* Rename `datasources` to `config_templates` in dataset manifest. [#570](https://github.com/elastic/package-registry/pull/570)
* Remove support for logs and metrics category. [#571](https://github.com/elastic/package-registry/pull/571)
* Remove `dataset.type: event` as suported type. [#567](https://github.com/elastic/package-registry/pull/567)
* Remove support for requirements. Use conditions instead. [#574](https://github.com/elastic/package-registry/pull/574)

### Added
* Use filepath.Walk to find valid package content data. [#438](https://github.com/elastic/package-registry/pull/438)
* Validate handlebarsjs stream configuration templates. [#445](https://github.com/elastic/package-registry/pull/445)
* Serve favicon as embedded resource. [#468](https://github.com/elastic/package-registry/pull/468)
* Generate index.json file. [#470](https://github.com/elastic/package-registry/pull/470)
* Stream archived package content. [#472](https://github.com/elastic/package-registry/pull/472)
* Generate package index.json files. [#479](https://github.com/elastic/package-registry/pull/479)
* Add validation for dataset type. [#501](https://github.com/elastic/package-registry/pull/501)
* Add -dry-run flag. [#503](https://github.com/elastic/package-registry/pull/503)
* Encode fields in Kibana objects if not encoded. [#506](https://github.com/elastic/package-registry/pull/506)
* Validate required fields in datasets. [#507](https://github.com/elastic/package-registry/pull/507)
* Do not require "config.yml". [#508](https://github.com/elastic/package-registry/pull/508)
* Validate version consistency. [#510](https://github.com/elastic/package-registry/pull/510)
* Remove package code generator. [#513](https://github.com/elastic/package-registry/pull/513)
* Support multiple packages paths. [#525](https://github.com/elastic/package-registry/pull/525)
* Added support for ecs style validation for dataset fields. [#520](https://github.com/elastic/package-registry/pull/520)
* Use BasePackage for search output data. [#529](https://github.com/elastic/package-registry/pull/529)
* Add support for owner field in package manifest. [#536](https://github.com/elastic/package-registry/pull/536)
* Introduce development mode. [#543](https://github.com/elastic/package-registry/pull/543)
* Add additional supported categories to package. [#533](https://github.com/elastic/package-registry/pull/533)
* Apply rule: first package found served. [#546](https://github.com/elastic/package-registry/pull/546)
* Implement package watcher. [#553](https://github.com/elastic/package-registry/pull/553)
* Add conditions as future replacement of requirements. [#519](https://github.com/elastic/package-registry/pull/519)
* Introduce `elasticsearch.ingest_pipeline.name` as config option. [#](https://github.com/elastic/package-registry/pull/)

### Deprecated

* Delete package index.json from archives. Don't serve index.json as resource. [#488](https://github.com/elastic/package-registry/pull/488)

## [v0.4.0](https://github.com/elastic/package-registry/compare/v0.3.0...v0.4.0)

### Breaking changes

* Change package path from /package/{packagename}-{version} to /package/{packagename}/{version} [#300](https://github.com/elastic/integrations-registry/pull/300)
* By default /search?package= now only returns the most recent package. [#301](https://github.com/elastic/integrations-registry/pull/301)
* Stream configuration filenames have `.hbs` suffix appended [#308](https://github.com/elastic/package-registry/pull/380)
* Align package storage directories with public dir structure [#376](https://github.com/elastic/package-registry/pull/376)
* Use index template v2 format for pre-built and generated index templates. [#392](https://github.com/elastic/package-registry/pull/392)

### Bugfixes

* Remove caching headers in case of errors. [#275](https://github.com/elastic/integrations-registry/pull/275)

### Added

* Allow to set cache times through config. [#271](https://github.com/elastic/integrations-registry/pull/271)
* Make README.md file a required file for a package. [#287](https://github.com/elastic/integrations-registry/pull/287)
*  Add stream fields to each dataset [#296](https://github.com/elastic/integrations-registry/pull/296)
* Add `all` query param to return all packages. By default is set false. [#301](https://github.com/elastic/integrations-registry/pull/301)
* Add `multiple` config for datasource. By default true. [#361](https://github.com/elastic/integrations-registry/pull/361)
* Add `removable` flag to package manifest. Default is true. [#359](https://github.com/elastic/integrations-registry/pull/359)
* Add stream template to package json. [#335](https://github.com/elastic/integrations-registry/pull/335)
* Add support for multiple inputs per dataset. [#346](https://github.com/elastic/integrations-registry/pull/346)
* Add experimental releases for packages and datasets. [#382](https://github.com/elastic/integrations-registry/pull/382)
* Handle invalid query params and return error. [#382](https://github.com/elastic/integrations-registry/pull/382)
* Add basic access logs. [#400](https://github.com/elastic/integrations-registry/pull/400)
* Validate ingest pipeline during packaging phrase. [#426](https://github.com/elastic/package-registry/pull/426)
* Use http.FileServer to serve package content and define HTTP headers [#425](https://github.com/elastic/package-registry/pull/425)
* Remove requirement for streams definition in dataset manifest. [#483](https://github.com/elastic/package-registry/pull/483)


## [v0.3.0](https://github.com/elastic/package-registry/compare/v0.2.0...v0.3.0)

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
