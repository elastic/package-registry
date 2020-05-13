# Kafka integration for Java consumers

This integration collects logs and metrics from Kafka Java consumers using
Jolokia.

## Compatibility

<!-- TODO: Add a link to Jolokia "input" in Metricbeat -->
The `consumer` metricset requires Jolokia to fetch JMX metrics. Refer to the Metricbeat documentation about Jolokia for more information.


## Metrics

### consumer

<!-- TODO example event -->

The fields reported are:

**Exported fields**

| Field | Description | Type |
|---|---|---|
| kafka.broker.address | Broker advertised address | keyword |
| kafka.broker.id | Broker id | long |
| kafka.consumer.bytes_consumed | The average number of bytes consumed for a specific topic per second | float |
| kafka.consumer.fetch_rate | The minimum rate at which the consumer sends fetch requests to a broker | float |
| kafka.consumer.in.bytes_per_sec | The rate of bytes coming in to the consumer | float |
| kafka.consumer.kafka_commits | The rate of offset commits to Kafka | float |
| kafka.consumer.max_lag | The maximum consumer lag | float |
| kafka.consumer.mbean | Mbean that this event is related to | keyword |
| kafka.consumer.messages_in | The rate of consumer message consumption | float |
| kafka.consumer.records_consumed | The average number of records consumed per second for a specific topic | float |
| kafka.consumer.zookeeper_commits | The rate of offset commits to ZooKeeper | float |
| kafka.partition.id | Partition id. | long |
| kafka.partition.topic_broker_id | Unique id of the partition in the topic and the broker. | keyword |
| kafka.partition.topic_id | Unique id of the partition in the topic. | keyword |
| kafka.topic.error.code | Topic error code. | long |
| kafka.topic.name | Topic name | keyword |
