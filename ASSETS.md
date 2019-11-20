# Package assets

This doc is to describe all the assets available in the packages and some details around each asset.

## General Assets

### /manifest.yml

The manifest file contains all the information about the package.

* Asset Path: `/manifest.yml`
* Documentation: <<link to definition-1.0.0/manifest.yml>>

### /changelog.yml

The changelog of a package contains always all previous changes and not only the one from the last major, minor, bugfix release. 

* Documentation: <<link to definition-1.0.0/changelog.yml>>

### /docs

The docs directory contains one file, the README.md file. It is written in Markdown.

## /img

The `img` directory is used to store images and icon. Icons are preferrably in .svg format.


### /fields/*.yml, /dataset/*/fields/*.yml

The fields directory contain the definitions of the fields which are used for the Elasticsearch index template and the
Kibana index pattern. Each `fields` directory can contain one or multiple *.yml file.

The directory normally exists inside the dataset directory but can also be on the global level for common fields.


## /elasticsearch

Elasticsearch assets are the assets which are loaded into Elasticsearch. All of them are inside `elasticsearch` directory.
This can be either directly under `/elasticsearch` or `/dataset/*/elasticsearch`.


### ./elasticserach/ingest-pipeline/*.json|*.yml

The [Elasticsearch ingest pipeline](https://www.elastic.co/guide/en/elasticsearch/reference/current/pipeline.html) contains
information on how the data should be processed. Multiple ingest pipelines can depend on each other thanks to the
[pipeline processor](https://www.elastic.co/guide/en/elasticsearch/reference/current/pipeline-processor.html). As during
package creation, the exact names given to the pipeline by the package manager is not know, we will need to use
some variables to reference. An example on this can be found [here](https://github.com/elastic/beats/blob/master/filebeat/module/elasticsearch/deprecation/ingest/pipeline.json#L24)
 in Beats. The package manager has to be able to understand this template language (we still need
 to decide what our template language is) and replace the pipeline ids with the correct values.
 
The pipelines can either be in json or yaml format.

 ### ./elasticsearch/index-template/*.json

The [Elasticsearch index template](https://www.elastic.co/guide/en/elasticsearch/reference/current/indices-templates.html)
is used to have a template applied to certain index patterns. Inside the Index Template the values `index_patterns` is defined
for the matching indices. As the indexing convention is either given by the index manager or the user, the package
manager must be able to overwrite the index pattern.

On the Beats side today the Index Template is generated out of the `fields.yml` files. This allows to give more flexibility
to generate the correct template for different Elasticsearch version. As we can release package packages for different
version of Elasticsearch independently this is probably not needed anymore. I expect fields.yml to stick around as it's a nice
way to create index templates and index patterns in one go. The package manager should be able to generate index templates
and index patterns out of all the combined fields.yml.

An Index Template also relates to the ILM policy as it can reference to which ILM policy should be applied to the indices
created.

As the index template is generated out of the fields.yml, it is not expected to exist in the package for now.

### ./elasticsearch/ilm-policy/*.json`

The Elasticsearch index lifecycle management policy can be added / removed through the [API](https://www.elastic.co/guide/en/elasticsearch/reference/master/index-lifecycle-management-api.html). For the ILM policy it's important
that the id / name of it matches what was configured in the index template.

The setup of ILM also requires to created an [alias and a write index](https://www.elastic.co/guide/en/elasticsearch/reference/current/indices-aliases.html#aliases-write-index). It's important that this happens before the first data is ingested. More details on this
can be found in the [rollover documentation](https://www.elastic.co/guide/en/elasticsearch/reference/master/indices-rollover-index.html).

Even if an ILM policy is created after a template and the write index were created, it will still apply. But if data is
ingested before the template and the write index exist, this will break the system.

### ./elasticsearch/rollup-job/*.json

A [rollup job](https://www.elastic.co/guide/en/elasticsearch/reference/current/rollup-apis.html) defines on how metric
data is rolled up. One special thing about rollup jobs is that they can only be created if index and data is already around.
Rollup jobs can potentially also be generated from `fields.yml` but it should be up to the package creator to do this.

Rollup jobs today do not support rollup templates which would be nice to have, see [discussion here](https://github.com/elastic/beats/pull/7220).
This would allow to separate creation of the template and actually creation / start of the job.

A rollup job depends on an index pattern and has a target index. The package manager should potentially be able
to configure this.

### ./elasticseearch/index/*.json

Index data is data that should be written into Elasticsearch. The data format is expected to be in the [Bulk format](https://www.elastic.co/guide/en/elasticsearch/reference/current/docs-bulk.html).

If the user can configure the index, the package manager should potentially be able to overwrite / prefix the index
fields inside the loaded data.

Loading of data can fail or partially fail. Because of this handling on failure must be possible.

### ./elasticsearch/ml-job/*.json`

Elasticsearch [Machine Learning Jobs](https://www.elastic.co/guide/en/elasticsearch/reference/current/ml-apis.html#ml-api-job-endpoint)
can be created in Elasticsearch assuming ML is enabled. As soon as a job is started, the job creates results. If results
are around, a Job can't be just removed anymore but also the results must be removed first (more details needed).

### Data Frames Transform

* Asset Path: `elasticsearch/data-frame-transform/*.json`

[Data Frame Transforms](https://www.elastic.co/guide/en/elasticsearch/reference/7.x/data-frame-apis.html) can be used to transfrom documents. There are a few things which are special about data frame transforms:

* Destination index must exist before creation
* Source index must be exist before creation
* If data frame uses ingest pipeline, it must exist before creation
* Data Fram transform must be stopped before deletion

Some of the above limitations might be removed in the future.



## Kibana

Kibana assets are the assets which are loaded in Kibana. The Kibana API docs can be found [here](https://www.elastic.co/guide/en/kibana/master/api.html).
A large portion of the Kibana assets are [saved objects](https://www.elastic.co/guide/en/kibana/master/saved-objects-api.html).
All saves objects are space aware, meaning the same object id with a different prefix can exists in multiple spaces.

Assuming the package manager generates the ids of the assets it must be capable to adjust the reference ids acrross
dashboards, visualizations, search, index patterns.

### ./kibana/dashboard/*.json

A Kibana dashboard consists of multiple visualisations it references.

### ./kibana/visualization/*.json

Visualizations are referenced inside dashboards and can reference a search object.

### ./kibana/search/*.json

The search object contains a saved search and is referenced by visualisations. A search object also references an index
like `"index": "filebeat-*"`. In case we allow users to adjust indices, this would have to be adjusted in the search object.

### ./kibana/index-pattern/*.json

The index pattern contains to information about the types of each field and additional settings for the fields like if it
is a percentage or the unit like seconds. Today in Beats the index pattern is generated out of the `fields.yml`. This
allows to generate the index pattern for different versions of Kibana. As we can release different versions of a package
we probably don't need this anymore.

ECS also provides a fields.yml in the same format. One limitation of index-patterns in Kibana is that they can't be extended
or support inheritance / composition like the index templates in Elasticsearch. Having many index patterns in Kibana is a
problem as a user would have to constantly switch between them. The package manager could work around this by
append / updating index pattern. But this will lead to the problem on how to remove these fields again.

### Infrastructure UI Source

* Asset Path: `kibana/infrastructure-ui-source/*.json`

The Infrastructure UI source is used to tell the Logs and Metrics UI which indices to query for data and how to 
visualise the data.

The asset is like dashboards / visualizations just a saved object and can be loaded the same way. But the Logs UI 
could also add an API for a tighter integration. At the moment there is no selection in the UI to change / switch the source
but it can be triggered through URL parameters.

### Space: ./kibana/space/*.json

The Kibana Space API can be found [here](https://www.elastic.co/guide/en/kibana/master/spaces-api.html). Kibana Spaces
are not saved objects and have their own API.

### Dataset

* Asset Path: `dataset/{dataset-name}/{package-structure}`

All dataset are defined inside the `dataset` directory. An example here is the `access` dataset of the `nginx` package.
Inside each dataset, the same structure is repeated which is defined for the overall package. In general ingest pipelines
and fields definitions are only expected inside dataset. An dataset is basically a template for an input.

**manifest.yml**

Each dataset must contain a manifest.yml. It contains all information about the dataset and how to configure it.

```
# Needs to describe the type of this input. Currently either metric or log
type: metric

# Each input can be in its own release status
release: beta

# If set to true, this will be enabled by default in the input selection
default: true

# Defines variables which are used in the config files and can be configured by the user / replaced by the package manager.
vars:
  -
    # Name of the variable that should be replaced
    name: hosts

    # Default value of the variable which is used in the UI and in the config if not specified
    default:
      ["http://127.0.0.1"]
    required: true

    # OS specific configurations!
    os.darwin:
      - /usr/local/var/log/nginx/error.log*
    os.windows:
      - c:/programdata/nginx/logs/error.log*


    # Below are UI Configs. Should we prefix these with ui.*?

    # Title used for the UI
    title: "Hosts lists"

    # Description of the varaiable which could be used in the UI
    description: Nginx hosts

    # A special type can be specified here for the UI Input document. By default it is just a 
    # text field.
    type: password

    required: true

  - name: period
    description: "Collection period. Valid values: 10s, 5m, 2h"
    default: "10s"
  - name: username
    type: text
  - name: password
    # This is the html input type?
    type: password


requirements:
  # Defines on which platform is input is available
  platform: ["linux", "freebsd"]
  elasticsearch.processors:
    # If a user does not have the user_agent processor, he should still be able to install the package but not
    # enable the access input
    - name: user_agent
      plugin: ingest-user-agent
    - name: geoip
      plugin: ingest-geoip

```

**fields**

The fields directory contains all fields.yml which are need to build the full template. All fields related to the dataset
must be in here in one or multiple files.

An open question is on how the fields for all the processors and autodiscovery are loaded.

**docs**

The docs for each dataset are combined with the overall docs. For the datasets it is encouraged to have `data.json` as an 
example event available.

**agent/input**

Agent input configuration for the input. It's by design not an array but a single entry. The package manager will build
a list out of it for the user.

**filebeat/input**

This contains the raw input configuration for the input.

**filebeat/module**

This contains the module configuration for the input. It is only 1 fileset and is not stored as an array.

**light_module**

This directory is designed to store light modules from Metricbeat. It contains the definition of the light module.

**module**

This contains the module configuration for this input. In the case of Metricbeat this means a module configuration with a
single metricset. By design it's not an array that is specified.

## Beats

How the input configuration for each Beat are stored still needs to be discussed.

## Definitions

### Package

A package is a list of assets for the Elastic stack that belong together. This can be ingest pipelines,
data sources, dashboards etc. All of these are defined above.

### Input

An input is the configuration that is sent to Beats / Agent to gather data. The input contains the infromation
on how to gather the data (e.g. log file) and where to send it (index + ingest pipeline). An example for a log
file might look as following:

```
inputs:
  - type: log
    paths: "/var/log/*.log"
    pipeline: log-pipeline
```

A similar example for docker metrics can look as below. But what we have below is the definition of 2 inputs, one 
for container metrics, one for cpu metrics:

```
  - type: metric/docker
    metricsets:
      - "container"
      - "cpu"
    hosts: ["unix:///var/run/docker.sock"]
    period: 10s
    pipeline: metric-docker-pipeline
```

### Data Source

A data source is a group of inputs. Each data source has a unique identifier and a name attached to it. A data
source for Apache could look as following:

```
datasource.name: apache
datasource.name: 4494ee18-2a5a-4212-afa7-9bbe9ade6bfc
inputs:
  - type: log
    paths: "/var/log/apache/access.log"
    pipeline: apache-access-pipeline
  - type: metric/apache
    metricsets:
      - "stats"
    period: 10s
```

The above is more a descriptive example for a data source and not necessarly on how it will be stored.
