# Integration package assets

This doc is to describe all the assets available in the integration packages and some details around each asset.

## General Assets

### Manifest

* Asset Path: `manifest.yml`

The `manifest.yml` contains the information about the pacakge. The most basic version looks as following:

```
name: envoyproxy
description: This is the envoyproxy integration.
version: 0.0.2
```

It will also contain definition with which versions of the Elastic Stack products its compatible and the requirements. 
(more to come).

### Fields.yml

* Asset Path: fields/*.yml

The fields.yml files are used for fields definitions and can be used to generate the index pattern in Kibana, elasticsearch
index template or rollup jobs. It's not clear yet on how the integrations manager should use this file and if.

The directory is reserved for multiple fields.yml as each integration, beat and ecs have it's own `fields.yml`.

## Elasticsearch

Elasticsearch assets are the assets which are loaded into Elasticsearch. All of them are inside `elasticsearch` directory.


### Ingest Pipeline

* Asset Path: `elasticsearch/ingest-pipeline/*.json`

The [Elasticsearch ingest pipeline](https://www.elastic.co/guide/en/elasticsearch/reference/current/pipeline.html) contains
information on how the data should be processed. Multiple ingest pipelines can depend on each other thanks to the
[pipeline processor](https://www.elastic.co/guide/en/elasticsearch/reference/current/pipeline-processor.html). As during 
package creation, the exact names given to the pipeline by the integrations manager is not know, we will need to use
some variables to reference. An example on this can be found [here](https://github.com/elastic/beats/blob/master/filebeat/module/elasticsearch/deprecation/ingest/pipeline.json#L24)
 in Beats. It means the integrations manager will have to be able to understand this template language (we still need 
 to decide what our template language is) and replace the pipeline ids with the correct values.
 
 ### Index Template

* Asset Path: `elasticsearch/index-template/*.json`

The [Elasticsearch index template](https://www.elastic.co/guide/en/elasticsearch/reference/current/indices-templates.html)
is used to have a template applied to certain index patterns. Inside the Index Template the values `index_patterns` is defined
for the matching indices. As the indexing convention is either given by the index manager or the user, the integrations
manager must be able to overwrite the index pattern.

On the Beats side today the Index Template is generated out of the `fields.yml` files. This allows to give more flexibility
to generate the correct template for different Elasticsearch version. As we can release integrations packages for different
version of Elasticsearch independently this is probably not needed anymore. I expect one or multiple fields.yml to be 
in each Integrations Package but leave it to the package creator to create the index template (TBD).

An Index Template also relates to the ILM policy as it can reference to which ILM policy should be applied to the indices
created.

### ILM Policy

* Asset Path: `elasticsearch/ilm-policy/*.json`

The Elasticsearch index lifecycle management policy can be added / removed through the [API](https://www.elastic.co/guide/en/elasticsearch/reference/master/index-lifecycle-management-api.html). For the ILM policy it's important
that the id / name of it matches what was configured in the index template.

The setup of ILM also requires to created an [alias and a write index](https://www.elastic.co/guide/en/elasticsearch/reference/current/indices-aliases.html#aliases-write-index). It's important that this happens before the first data is ingested. More details on this
can be found in the [rollover documentation](https://www.elastic.co/guide/en/elasticsearch/reference/master/indices-rollover-index.html).

Even if an ILM policy is created after a template and the write index were created, it will still apply. But if data is
ingested before the template and the write index exist, this will break the system.

### Rollup Job

* Asset Path: `elasticsearch/rollup-job/*.json`

A [rollup job](https://www.elastic.co/guide/en/elasticsearch/reference/current/rollup-apis.html) defines on how metric 
data is rolled up. One special thing about rollup jobs is that they can only be created if index and data is already around.
Rollup jobs can potentially also be generated from `fields.yml` but it should be up to the package creator to do this.

Rollup jobs today do not support rollup templates which would be nice to have, see [discussion here](https://github.com/elastic/beats/pull/7220).
This would allow to separate creation of the template and actually creation / start of the job.

A rollup job depends on an index pattern and has a target index. The integrations manager should potentially be able
to configure this.

### Index Data

* Asset Path: `elasticseearch/index/*.json`

Index data is data that should be written into Elasticsearch. The data format is expected to be in the [Bulk format](https://www.elastic.co/guide/en/elasticsearch/reference/current/docs-bulk.html).

If the user can configure the index, the integrations manager should potentially be able to overwrite / prefix the index
fields inside the loaded data.

Loading of data can fail or partially fail. Because of this handling on failure must be possible.

### ML Jobs

* Asset Path: `elasticsearch/ml-job/*.json`

Elasticsearch [Machine Learning Jobs](https://www.elastic.co/guide/en/elasticsearch/reference/current/ml-apis.html#ml-api-job-endpoint)
can be created in Elasticsearch assuming ML is enabled. As soon as a job is started, the job creates results. If results
are around, a Job can't be just removed anymore but also the results must be removed first (more details needed).

## Kibana

Kibana assets are the assets which are loaded in Kibana. The Kibana API docs can be found [here](https://www.elastic.co/guide/en/kibana/master/api.html).
A large portion of the Kibana assets are [saved objects](https://www.elastic.co/guide/en/kibana/master/saved-objects-api.html).
All saves objects are space aware, meaning the same object id with a different prefix can exists in multiple spaces.

Assuming the integrations manager generates the ids of the assets it must be capable to adjust the reference ids acrross
dashboards, visualizations, search, index patterns.

### Dashboard

* Asset Path: `kibana/dashboard/*.json`

A Kibana dashboard consists of multiple visualisations it references.

### Visualization

* Asset Path: `kibana/visualization/*.json`

Visualizations are referenced inside dashboards and can reference a search object.

### Search

* Asset Path: `kibana/search/*.json`

The search object contains a saved search and is referenced by visualisations. A search object also references an index
like `"index": "filebeat-*"`. In case we allow users to adjust indices, this would have to be adjusted in the search object.

### Index Pattern

* Asset Path: `kibana/index-pattern/*.json`

The index pattern contains to information about the types of each field and additional settings for the fields like if it
is a percentage or the unit like seconds. Today in Beats the index pattern is generated out of the `fields.yml`. This
allows to generate the index pattern for different versions of Kibana. As we can release different versions of a package
we probably don't need this anymore.

ECS also provides a fields.yml in the same format. One limitation of index-patterns in Kibana is that they can't be extended
or support inheritance / composition like the index templates in Elasticsearch. Having many index patterns in Kibana is a 
problem as a user would have to constantly switch between them. The integrations manager could work around this by
append / updating index pattern. But this will lead to the problem on how to remove these fields again.

### Space

* Asset Path: `kibana/space/*.json`

The Kibana Space API can be found [here](https://www.elastic.co/guide/en/kibana/master/spaces-api.html). Kibana Spaces
are not saved objects and have their own API.


## Beats

How the input configuration for each Beat are stored still needs to be discussed.
