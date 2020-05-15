# Kafka integration for Java producers.

This integration collects logs and metrics from Kafka Java producers using
Jolokia.

## Compatibility

<!-- TODO: Add a link to Jolokia "input" in Metricbeat -->
The `producer` metricset requires Jolokia to fetch JMX metrics. Refer to the Metricbeat documentation about Jolokia for more information.


## Metrics

### producer

An example event of the `producer` dataset looks as following:

```$json
{
  "@timestamp": "2020-05-15T15:30:03.990Z",
  "service": {
    "address": "localhost:8777",
    "type": "kafka"
  },
  "event": {
    "dataset": "kafka.producer",
    "module": "kafka",
    "duration": 9662361
  },
  "metricset": {
    "name": "producer",
    "period": 10000
  },
  "kafka": {
    "producer": {
      "mbean": "kafka.producer:client-id=console-producer,type=producer-metrics",
      "request_rate": 0.03333333333333333,
      "record_send_rate": 0,
      "response_rate": 0.03333333333333333,
      "record_error_rate": 0,
      "io_wait": 1413696731.5789473,
      "available_buffer_bytes": 33554432,
      "record_retry_rate": 0
    }
  },
  "stream": {
    "namespace": "default",
    "type": "metrics",
    "dataset": "kafka_java_producer.producer"
  },
  "agent": {
    "type": "metricbeat",
    "version": "8.0.0",
    "ephemeral_id": "178ff0e9-e3dd-4bdf-8e3d-8f67a6bd72ef",
    "id": "5aba67f2-2050-4d19-8953-ba20f0a5483c",
    "name": "voyager"
  },
  "ecs": {
    "version": "1.5.0"
  }
}
```

The fields reported are:

**Exported fields**

| Field | Description | Type |
|---|---|---|
| kafka.broker.address | Broker advertised address | keyword |
| kafka.broker.id | Broker id | long |
| kafka.partition.id | Partition id. | long |
| kafka.partition.topic_broker_id | Unique id of the partition in the topic and the broker. | keyword |
| kafka.partition.topic_id | Unique id of the partition in the topic. | keyword |
| kafka.producer.available_buffer_bytes | The total amount of buffer memory | float |
| kafka.producer.batch_size_avg | The average number of bytes sent | float |
| kafka.producer.batch_size_max | The maximum number of bytes sent | long |
| kafka.producer.io_wait | The producer I/O wait time | float |
| kafka.producer.mbean | Mbean that this event is related to | keyword |
| kafka.producer.message_rate | The producer message rate | float |
| kafka.producer.out.bytes_per_sec | The rate of bytes going out for the producer | float |
| kafka.producer.record_error_rate | The average number of retried record sends per second | float |
| kafka.producer.record_retry_rate | The average number of retried record sends per second | float |
| kafka.producer.record_send_rate | The average number of records sent per second | float |
| kafka.producer.record_size_avg | The average record size | float |
| kafka.producer.record_size_max | The maximum record size | long |
| kafka.producer.records_per_request | The average number of records sent per second | float |
| kafka.producer.request_rate | The number of producer requests per second | float |
| kafka.producer.response_rate | The number of producer responses per second | float |
| kafka.topic.error.code | Topic error code. | long |
| kafka.topic.name | Topic name | keyword |

