
# Long docs integration

This is the long docs integration that is focused on containing as many different documentation blocks as possible

## Caveats, or testing italic

This integration is considered to be in _beta_.

## Test a link

This is a [link](https://github.com/elastic/integrations-registry) inside the docs.

## Examples from a file

The following file shows how an event might look like:

```
{{ > ./data.json }}
```

## Template parts

Some part of our documentation will require templated documentation. An example of this is to link to the download links of the Beat with the same version as Kibana. The link below with the link text is an example:

[https://artifacts.elastic.co/downloads/beats/filebeat/filebeat-{{stack.version}}-linux-x86_64.tar.gz](https://artifacts.elastic.co/downloads/beats/filebeat/filebeat-{{stack.version}}-linux-x86_64.tar.gz)

# Below are more docs parts

The below docs are for now copied from CoreDNS. More special cases should be added over time.

## Download and install Filebeat

Grab the filebeat binary from elastic.co, and install it by following the instructions.

## Deployment Scenario #1: coredns native deployment

Make sure to update coredns configuration to enable log plugin. This module assumes that coredns log
entries will be written to /var/log/coredns.log. Should it be not the case, please point the module 
log path to the path of the log file. 

Update filebeat.yml to point to Elasticsearch and Kibana. 
Setup Filebeat.

```
./filebeat setup --modules coredns -e
```

Enable the Filebeat coredns module
```
./filebeat modules enable coredns
```

Start Filebeat
```
./filebeat -e
```

Now, the Coredns logs and dashboard should appear in Kibana.


## Deployment Scenario #2: coredns for kubernetes 

For Kubernetes deployment, the filebeat daemon-set yaml file needs to be deployed to the 
Kubernetes cluster. Sample configuration files is provided under the `beats/deploy/filebeat` 
directory, and can be deployed by doing the following:
```
kubectl apply -f filebeat
```

#### Note the following section in the ConfigMap, make changes to the yaml file if necessary
```
  filebeat.autodiscover:
    providers:
      - type: kubernetes
        hints.enabled: true
        hints.default_config.enabled: false

  processors:
    - add_kubernetes_metadata:
        in_cluster: true
```

This enables auto-discovery and hints for filebeat. When default.disable is set to true (default value is false), it will disable log harvesting for the pod/container, unless it has specific annotations enabled. This gives users more granular control on kubernetes log ingestion. The `add_kubernetes_metadata` processor will add enrichment data for Kubernetes to the ingest logs.

#### Note the following section in the DaemonSet, make changes to the yaml file if necessary
```
apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: filebeat
  namespace: kube-system
  labels:
    k8s-app: filebeat
spec:
  template:
    metadata:
      labels:
        k8s-app: filebeat
    spec:
      serviceAccountName: filebeat
      terminationGracePeriodSeconds: 30
      containers:
      - name: filebeat
        image: docker.elastic.co/beats/filebeat:%VERSION%
        args: [
          "sh", "-c", "filebeat setup -e --modules coredns -c /etc/filebeat.yml && filebeat -e -c /etc/filebeat.yml"
        ]
        env:
        # Edit the following values to reflect your setup accordingly
        - name: ELASTICSEARCH_HOST
          value: 192.168.99.1
        - name: ELASTICSEARCH_USERNAME
          value: elastic
        - name: ELASTICSEARCH_PASSWORD
          value: changeme
        - name: KIBANA_HOST
          value: 192.168.99.1
```

The module setup step can also be done separately without Kubernetes if applicable, and in that case, the args can be simplified to:
```
        args: [
          "sh", "-c", "filebeat -e -c /etc/filebeat.yml"
        ]
```

### Note that you probably need to update the coredns configmap to enable logging, and coredns deployment to add proper annotations. 

##### Sample ConfigMap for coredns:

```
apiVersion: v1
data:
  Corefile: |
    .:53 {
        log
        errors
        health
        kubernetes cluster.local in-addr.arpa ip6.arpa {
           pods verified
           endpoint_pod_names
           upstream
           fallthrough in-addr.arpa ip6.arpa
        }
        prometheus :9153
        proxy . /etc/resolv.conf
        cache 30
        loop
        reload
        loadbalance
    }
kind: ConfigMap
metadata:
  creationTimestamp: "2019-01-31T21:02:57Z"
  name: coredns
  namespace: kube-system
  resourceVersion: "185717"
  selfLink: /api/v1/namespaces/kube-system/configmaps/coredns
  uid: 95a5d5cb-259b-11e9-8e5d-080027971f3c
```

#### Sample Deployment for coredns. Note the annotations.

```
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: coredns
spec:
  replicas: 2
  template:
    metadata:
      annotations:
        "co.elastic.logs/module": "coredns"
        "co.elastic.logs/fileset": "log"
        "co.elastic.logs/disable": "false"
      labels:
        k8s-app: coredns
    spec:
      <snipped>
```

