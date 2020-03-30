# MySQL Integration

This integration periodically fetches logs and metrics from [https://www.mysql.com/](MySQL) servers.

## Compatibility

The `error` and `slowlog` datasets were tested with logs from MySQL 5.5, 5.7 and 8.0, MariaDB 10.1, 10.2 and 10.3, and Percona 5.7 and 8.0.

The `galera_status` and `status` datasets were tested with MySQL and Percona 5.7 and 8.0 and are expected to work with all
versions >= 5.7.0. It is also tested with MariaDB 10.2, 10.3 and 10.4.

## Logs

### error

The `error` dataset collects the MySQL error logs.

**Exported fields**

| Field                       	| Description                                                                                                                                                                                            	| Type 	|
|-----------------------------	|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------	|------	|
| nginx.access.remote_ip_list 	| An array of remote IP addresses. It is a list because it is common to include, besides the client IP address, IP addresses from headers like X-Forwarded-For. Real source IP is restored to source.ip. 	| ip   	|


### slowlog

The `slowlog` dataset collects the MySQL slow logs.

**Exported fields**

| Field                     	| Description                                                                    	| Type 	|
|---------------------------	|--------------------------------------------------------------------------------	|------	|
| nginx.error.connection_id 	| Connection identifier.                                                         	| ip   	|

## Metrics

### galera_status

The `galera_status` dataset periodically fetches metrics from [http://galeracluster.com/](Galera)-MySQL cluster servers.

An example event for `galera_status` looks as following:

```$json
{
    "@timestamp":"2016-05-23T08:05:34.853Z",
    "agent": {
        "hostname": "host.example.com",
        "name": "host.example.com"
    },
    "event": {
        "dataset": "mysql.galera_status",
        "duration": 115000
    },
    "metricset": {
        "name": "galera_status"
    },
    "mysql":{
        "galera_status":{
            "apply": {
                "oooe": 0,
                "oool": 0,
                "window": 1
            },
            "connected": "ON",
            "flow_ctl": {
                "recv": 0,
                "sent": 0,
                "paused": 0,
                "paused_ns": 0
            },
            "ready": "ON",
            "received": {
                "count": 173,
                "bytes": 152425
            },
            "local": {
                "state": "Synced",
                "bf_aborts": 0,
                "cert_failures": 0,
                "commits": 1325,
                "recv": {
                    "queue_max": 2,
                    "queue_min": 0,
                    "queue": 0,
                    "queue_avg": 0.011561
                },
                "replays": 0,
                "send": {
                    "queue_min": 0,
                    "queue": 0,
                    "queue_avg": 0,
                    "queue_max": 1
                }
            },
            "evs": {
                "evict": "",
                "state": "OPERATIONAL"
            },
            "repl": {
                "bytes": 1689804,
                "data_bytes": 1540647,
                "keys": 4170,
                "keys_bytes": 63973,
                "other_bytes": 0,
                "count": 1331
            },
            "commit": {
                "oooe": 0,
                "window": 1
            },
            "cluster": {
                "conf_id": 930,
                "size": 3,
                "status": "Primary"
            },
            "last_committed": 23944,
            "cert": {
                "deps_distance": 43.524557,
                "index_size": 22,
                "interval": 0
            }
        }
    }
}
```

The fields reported are:

| Field                     	| Description                                                                    	| Type    	|
|---------------------------	|--------------------------------------------------------------------------------	|---------	|
| nginx.stubstatus.hostname 	| Nginx hostname.                                                                	| keyword 	|

### status

The MySQL `status` dataset collects data from MySQL by running a `SHOW GLOBAL STATUS;` SQL query. This query returns a large number of metrics.

An example event for `status` looks as following:

```$json
{
    "@timestamp":"2016-05-23T08:05:34.853Z",
    "agent": {
        "hostname": "host.example.com",
        "name": "host.example.com"
    },
    "event": {
        "dataset": "mysql.status",
        "duration": 115000
    },
    "metricset": {
        "name": "status"
    },
    "mysql": {
        "status": {
            "aborted": {
                "clients": 3,
                "connects": 4
            },
            "binlog": {
                "cache": {
                    "disk_use": 0,
                    "use": 0
                }
            },
            "bytes": {
                "received": 1272,
                "sent": 47735
            },
            "command": {
                "delete": 0,
                "insert": 0,
                "select": 1,
                "update": 0
            },
            "connections": 12,
            "created": {
                "tmp": {
                    "disk_tables": 0,
                    "files": 5,
                    "tables": 6
                }
            },
            "delayed": {
                "errors": 0,
                "insert_threads": 0,
                "writes": 0
            },
            "flush_commands": 1,
            "handler": {
                "commit": 0,
                "delete": 0,
                "external_lock": 140,
                "mrr_init": 0,
                "prepare": 0,
                "read": {
                    "first": 3,
                    "key": 2,
                    "last": 0,
                    "next": 32,
                    "prev": 0,
                    "rnd": 0,
                    "rnd_next": 1728
                },
                "rollback": 0,
                "savepoint": 0,
                "savepoint_rollback": 0,
                "update": 0,
                "write": 1705
            },
            "innodb": {
                "buffer_pool": {
                    "bytes": {
                        "data": 6914048,
                        "dirty": 0
                    },
                    "pages": {
                        "data": 422,
                        "dirty": 0,
                        "flushed": 207,
                        "free": 7768,
                        "misc": 1,
                        "total": 8191
                    },
                    "pool": {
                        "reads": 423,
                        "wait_free": 0
                    },
                    "read": {
                        "ahead": 0,
                        "ahead_evicted": 0,
                        "ahead_rnd": 0,
                        "requests": 14198
                    },
                    "write_requests": 207
                }
            },
            "max_used_connections": 3,
            "open": {
                "files": 16,
                "streams": 0,
                "tables": 60
            },
            "opened_tables": 67,
            "queries": 10,
            "questions": 9,
            "threads": {
                "cached": 0,
                "connected": 3,
                "created": 3,
                "running": 1
            }
        }
    }
}
```

The fields reported are:

| Field                     	| Description                                                                    	| Type    	|
|---------------------------	|--------------------------------------------------------------------------------	|---------	|
| nginx.stubstatus.hostname 	| Nginx hostname.                                                                	| keyword 	|
