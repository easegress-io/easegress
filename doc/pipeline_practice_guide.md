# Ease Gateway Pipeline Practice Guide

## Overview

In this document, we would like to introduce 6 pipelines from real practice to cover some worth usecases as examples.

| Name | Description | Complexity level |
|:--|:--|:--:|
| [Ease Monitor edge service](#ease-monitor-edge-service) | Runs an example HTTPS endpoint to receive an Ease Monitor data, processes it in the pipeline and sends prepared data to kafka finally. | Beginner |
| [HTTP traffic throttling](#http-traffic-throttling) | Performs latency and throughput rate based traffic control. | Beginner |
| [Service circuit breaking](#service-circuit-breaking) | As a protection function, once the service failures reach a certain threshold all further calls to the service will be returned with an error directly, and when the service recovery the breaking function will be disabled automatically. | Beginner |
| [HTTP proxy with caching](#http-proxy-with-caching) | Caches HTTP/HTTPS response for duplicated request | Intermediate |
| [Service downgrading to protect critical service](#service-downgrading-to-protect-critical-service) | Under unexpected taffic which higher than planed, sacrifice the unimportant services but keep critical request is handled. | Intermediate |
| [Flash sale event support](#flash-sale-event-support) | A pair of pipelines to support flash sale event. For e-Commerence, it means we have very low price items with limited stock, but have huge amount of people online compete on that. | Advanced |

## Ease Monitor edge service

In this case, we will prepare a piplein to runs necessary processes as Ease Monitor edge service endpoint.

### Plugin

1. [HTTP input](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#http-input-plugin): To enable HTTPS endpoint to receive Ease Monitor data send from the client.
2. [IO reader](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#io-reader-plugin): To read Ease Monitor data from the client via HTTPS transport layer in to local memory for handling in next steps.
3. [Json validator](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#json-validator-plugin): Validating the Ease Monitor data send from the client is using a certain schema. You can use [Ease Monitor graphite validator](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#ease-monitor-graphite-validator-plugin) if you would like to use the pipeline to handle Ease Monitor data with graphite plaintext protocol.
4. [Ease Monitor Json GID extractor](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#ease-monitor-json-gid-extractor-plugin): Extracts Ease Monitor global ID from the Ease Monitor data. You can use [Ease Monitor graphite GID extractor](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#ease-monitor-graphite-gid-extractor-plugin) if you would like to use the pipeline to handle Ease Monitor data with graphite plaintext protocol.
5. [Kafka output](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#kafka-output-plugin): Sending the data to configured kafka topic, Ease Monitor pipeline will fetch them for rest of processes.

Using follow [Administration API](https://github.com/hexdecteam/easegateway/blob/master/doc/admin_api_ref.swagger.yaml) calls to setup above plugins:

```
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPInput", "config": {"plugin_name": "test-httpinput", "url": "/test", "method": "POST", "headers_enum": {"name": ["bar", "bar1"]}, "request_body_io_key": "HTTP_REQUEST_BODY_IO"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "IOReader", "config": {"plugin_name": "test-ioreader", "input_key":"HTTP_REQUEST_BODY_IO", "output_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "JSONValidator", "config": {"plugin_name": "test-jsonvalidator", "schema": "{\"title\": \"Record\",\"type\": \"object\",\"properties\": {\"name\": {\"type\": \"string\"}}, \"required\": [\"name\"]}", "data_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "EaseMonitorJSONGidExtractor", "config": {"plugin_name": "test-jsongidextractor", "gid_key": "GID", "data_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "KafkaOutput", "config": {"plugin_name": "test-kafkaoutput", "topic": "test", "brokers": ["192.168.98.130:9092"], "message_key_key": "GID", "data_key": "DATA"}}'
```

### Pipeline

You can use follow [Administration API](https://github.com/hexdecteam/easegateway/blob/master/doc/admin_api_ref.swagger.yaml) calls to setup the pipeline:

```
$ curl http://127.0.0.1:9090/admin/v1/pipelines -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "LinearPipeline", "config": {"pipeline_name": "test-jsonpipeline", "plugin_names": ["test-httpinput", "test-ioreader", "test-jsonvalidator", "test-jsongidextractor", "test-kafkaoutput"], "parallelism": 10}}'
```

### Test

Once the pipeline is created (above Administration API returns HTTP 200), any incoming Ease Monitor data send to the URL with certain method and header will be handled in pipeline and send to the kafka topic. You might send test requests by blow commands. The data in `~/load` file just show you an example, refer Ease Monitor document to check complete data.

```
$ cat ~/load
{
        "name": "test-workload",
        "system": "ExampleSystem",
        "application": "ExampleApplication",
        "instance": "ExampleInstance",
        "hostname": "ExampleHost",
        "hostipv4": "192.168.98.130"
}
$ LOAD=`cat ~/load`
$ curl -i -k https://127.0.0.1:10443/test -X POST -i -w "\n" -H "name:bar" -d "$LOAD"
```

## HTTP traffic throttling

Currently Ease Gateway supports two kinds of traffic throttling:

* Throughput rate based throttling. This kind traffic throttling provides a clear and predictable workload limitation on the upstream, any exceeded requests will all be rejected by a ResultFlowControl internal error. Finally the error will be translate to a special response to the client, for example, Http Input plugin translates the error to HTTP status code 429 (StatusTooManyRequests, RFC 6585.4). There are two potential limitations on this traffic throttling way:
	* When upstream is deployed in a distributed environment, it is hard to setup a accurate throughput rate limitation according to a single service instance due to reqeusts on the upstream could be passed by different instance.
	* When Ease Gateway is deployed in a distributed environment and more than one gateway instances are pointed to the same upstream service, under this case the requets could be handled by different gateway instance, so a clear throughput rate limitation in a single gateway instance does not really work on upstream.
* Upstream latency based throttling. This kind of traffic throttling provides a sliding window based on upstream handling latency, just like TCP flow control implementation the size of sliding window will be adjusted according to the case of upstream response, lower latency generally means better performance upstream has and more requests could be sent to upstream quickly. This traffic throttling way resolves above two potential limitations of throughput rate based throttling method.

### Throughput rate based throttling

In this case, we will prepare a piplein to show you how to setup a throughput rate limitation for Ease Monitor edge service. You can see we only need to create a plugin and add it to a certain position in to the pipeline easily.

#### Plugin

1. [HTTP input](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#http-input-plugin): To enable HTTPS endpoint to receive Ease Monitor data send from the client.
2. [Throughput rate limiter](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#throughput-rate-limiter-plugin): To add a limitation to control request rate. The limitation will be performed no matter how many concurrent clients send the request. In this case, we ask there are no more than 11 requests per second send to Ease Monitor pipeline.
3. [IO reader](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#io-reader-plugin): To read Ease Monitor data from the client via HTTPS transport layer in to local memory for handling in next steps.
4. [Json validator](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#json-validator-plugin): Validating the Ease Monitor data send from the client is using a certain schema.
5. [Ease Monitor Json GID extractor](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#ease-monitor-json-gid-extractor-plugin): Extracts Ease Monitor global ID from the Ease Monitor data. You can use [Graphite GID extractor](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#graphite-gid-extractor-plugin) if you would like to use the pipeline to handle Ease Monitor data with graphite plaintext protocol.
6. [Kafka output](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#kafka-output-plugin): Sending the data to configured kafka topic, Ease Monitor pipeline will fetch them for rest of processes.

Using follow [Administration API](https://github.com/hexdecteam/easegateway/blob/master/doc/admin_api_ref.swagger.yaml) calls to setup above plugins:

```
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPInput", "config": {"plugin_name": "test-httpinput", "url": "/test", "method": "POST", "headers_enum": {"name": ["bar", "bar1"]}, "request_body_io_key": "HTTP_REQUEST_BODY_IO"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "ThroughputRateLimiter", "config": {"plugin_name": "test-throughputratelimiter", "tps": "11"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "IOReader", "config": {"plugin_name": "test-ioreader", "input_key":"HTTP_REQUEST_BODY_IO", "output_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "JSONValidator", "config": {"plugin_name": "test-jsonvalidator", "schema": "{\"title\": \"Record\",\"type\": \"object\",\"properties\": {\"name\": {\"type\": \"string\"}}, \"required\": [\"name\"]}", "data_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "EaseMonitorJSONGidExtractor", "config": {"plugin_name": "test-jsongidextractor", "gid_key": "GID", "data_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "KafkaOutput", "config": {"plugin_name": "test-kafkaoutput", "topic": "test", "brokers": ["192.168.98.130:9092"], "message_key_key": "GID", "data_key": "DATA"}}'
```

#### Pipeline

We need to set throughput rate limiter plugin to a certain position in the pipeline, generally we should terminate request as earlier as possible in the pipeline since once limiation is reached there is no reason to handle the request for rest steps, in this case we add the limiter close to HTTP input plugin.

You can use follow [Administration API](https://github.com/hexdecteam/easegateway/blob/master/doc/admin_api_ref.swagger.yaml) calls to setup the pipeline:

```
$ curl http://127.0.0.1:9090/admin/v1/pipelines -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "LinearPipeline", "config": {"pipeline_name": "test-jsonpipeline", "plugin_names": ["test-httpinput", "test-throughputratelimiter", "test-ioreader", "test-jsonvalidator", "test-jsongidextractor", "test-kafkaoutput"], "parallelism": 10}}'
```

#### Test

```
$ cat ~/load
{
        "name": "test-workload",
        "system": "ExampleSystem",
        "application": "ExampleApplication",
        "instance": "ExampleInstance",
        "hostname": "ExampleHost",
        "hostipv4": "192.168.98.130"
}
$ LOAD=`cat ~/load`
$ ab -n 100 -c 20 -H "name:bar" -T "application/json" -p ~/load -f TSL1.2 https://127.0.0.1:10443/test
This is ApacheBench, Version 2.3 <$Revision: 1528965 $>
Copyright 1996 Adam Twiss, Zeus Technology Ltd, http://www.zeustech.net/
Licensed to The Apache Software Foundation, http://www.apache.org/

Benchmarking 127.0.0.1 (be patient).....done


Server Software:
Server Hostname:        127.0.0.1
Server Port:            10443
SSL/TLS Protocol:       TLSv1.2,ECDHE-RSA-AES128-GCM-SHA256,2048,128

Document Path:          /test
Document Length:        0 bytes

Concurrency Level:      20
Time taken for tests:   8.926 seconds
Complete requests:      100
Failed requests:        0
Total transferred:      9700 bytes
Total body sent:        33700
HTML transferred:       0 bytes
Requests per second:    11.20 [#/sec] (mean)
Time per request:       1785.239 [ms] (mean)
Time per request:       89.262 [ms] (mean, across all concurrent requests)
Transfer rate:          1.06 [Kbytes/sec] received
                        3.69 kb/s sent
                        4.75 kb/s total

Connection Times (ms)
              min  mean[+/-sd] median   max
Connect:        4   49  76.3      5     265
Processing:     9 1546 1232.0   1096    5594
Waiting:        1 1544 1233.5   1095    5593
Total:         34 1595 1228.1   1101    5599

Percentage of the requests served within a certain time (ms)
  50%   1101
  66%   1998
  75%   2000
  80%   2530
  90%   3328
  95%   3800
  98%   5596
  99%   5599
 100%   5599 (longest request)
```

### Upstream latency based throttling

In this case, we will prepare a piplein to show you how to setup a latency based limitation for Ease Monitor edge service. You can see we only need to create a plugin and add it to a certain position in to the pipeline easily, it just like what we did in throughput rate based throttling above.

#### Plugin

1. [HTTP input](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#http-input-plugin): To enable HTTPS endpoint to receive Ease Monitor data send from the client.
2. [Latency based sliding window limiter](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#latency-based-sliding-window-limiter-plugin): To add a limitation to control request rate. The limitation will be performed no matter how many concurrent clients send the request. In this case, we ask there are no more than 11 requests per second send to Ease Monitor pipeline.
3. [IO reader](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#io-reader-plugin): To read Ease Monitor data from the client via HTTPS transport layer in to local memory for handling in next steps.
4. [Json validator](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#json-validator-plugin): Validating the Ease Monitor data send from the client is using a certain schema.
5. [Ease Monitor Json GID extractor](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#ease-monitor-json-gid-extractor-plugin): Extracts Ease Monitor global ID from the Ease Monitor data. You can use [Graphite GID extractor](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#graphite-gid-extractor-plugin) if you would like to use the pipeline to handle Ease Monitor data with graphite plaintext protocol.
6. [Kafka output](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#kafka-output-plugin): Sending the data to configured kafka topic, Ease Monitor pipeline will fetch them for rest of processes.

Using follow [Administration API](https://github.com/hexdecteam/easegateway/blob/master/doc/admin_api_ref.swagger.yaml) calls to setup above plugins:

```
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPInput", "config": {"plugin_name": "test-httpinput", "url": "/test", "method": "POST", "headers_enum": {"name": ["bar", "bar1"]}, "request_body_io_key": "HTTP_REQUEST_BODY_IO"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "LatencyWindowLimiter", "config": {"plugin_name": "test-latencywindowlimiter", "latency_threshold_msec": 150, "plugins_concerned": ["test-kafkaoutput"], "window_size_max": 10, "windows_size_init": 5}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "IOReader", "config": {"plugin_name": "test-ioreader", "input_key":"HTTP_REQUEST_BODY_IO", "output_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "JSONValidator", "config": {"plugin_name": "test-jsonvalidator", "schema": "{\"title\": \"Record\",\"type\": \"object\",\"properties\": {\"name\": {\"type\": \"string\"}}, \"required\": [\"name\"]}", "data_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "EaseMonitorJSONGidExtractor", "config": {"plugin_name": "test-jsongidextractor", "gid_key": "GID", "data_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "KafkaOutput", "config": {"plugin_name": "test-kafkaoutput", "topic": "test", "brokers": ["192.168.98.130:9092"], "message_key_key": "GID", "data_key": "DATA"}}'
```

#### Pipeline

We need to set the limiter plugin to a certain position in the pipeline, generally we should terminate request as earlier as possible in the pipeline since once limiation is reached there is no reason to handle the request for rest steps, in this case we add the limiter close to HTTP input plugin.

You can use follow [Administration API](https://github.com/hexdecteam/easegateway/blob/master/doc/admin_api_ref.swagger.yaml) calls to setup the pipeline:

```
$ curl http://127.0.0.1:9090/admin/v1/pipelines -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "LinearPipeline", "config": {"pipeline_name": "test-jsonpipeline", "plugin_names": ["test-httpinput", "test-latencywindowlimiter", "test-ioreader", "test-jsonvalidator", "test-jsongidextractor", "test-kafkaoutput"], "parallelism": 10}}'
```

#### Test

```
$ ab -n 100 -c 20 -H "name:bar" -T "application/json" -p ~/load -f TSL1.2 https://127.0.0.1:10443/test
This is ApacheBench, Version 2.3 <$Revision: 1528965 $>
Copyright 1996 Adam Twiss, Zeus Technology Ltd, http://www.zeustech.net/
Licensed to The Apache Software Foundation, http://www.apache.org/

Benchmarking 127.0.0.1 (be patient)...apr_pollset_poll: The timeout specified has expired (70007)
$ ab -n 100 -c 20 -H "name:bar" -T "application/json" -p ~/load -f TSL1.2 https://127.0.0.1:10443/test
This is ApacheBench, Version 2.3 <$Revision: 1528965 $>
Copyright 1996 Adam Twiss, Zeus Technology Ltd, http://www.zeustech.net/
Licensed to The Apache Software Foundation, http://www.apache.org/

Benchmarking 127.0.0.1 (be patient).....done


Server Software:
Server Hostname:        127.0.0.1
Server Port:            10443
SSL/TLS Protocol:       TLSv1.2,ECDHE-RSA-AES128-GCM-SHA256,2048,128

Document Path:          /test
Document Length:        0 bytes

Concurrency Level:      20
Time taken for tests:   0.754 seconds
Complete requests:      100
Failed requests:        0
Total transferred:      9700 bytes
Total body sent:        33700
HTML transferred:       0 bytes
Requests per second:    132.65 [#/sec] (mean)
Time per request:       150.772 [ms] (mean)
Time per request:       7.539 [ms] (mean, across all concurrent requests)
Transfer rate:          12.57 [Kbytes/sec] received
                        43.66 kb/s sent
                        56.22 kb/s total

Connection Times (ms)
              min  mean[+/-sd] median   max
Connect:        4   53  40.6     40     169
Processing:    25   88  28.6     89     157
Waiting:        2   75  37.7     80     151
Total:         55  141  54.0    135     322

Percentage of the requests served within a certain time (ms)
  50%    135
  66%    141
  75%    152
  80%    169
  90%    213
  95%    272
  98%    322
  99%    322
 100%    322 (longest request)
```

## Service circuit breaking

In this case, we will prepare a piplein to show you how to add such a service circuit breaking mechanism to Ease Monitor edge service. You can see we only need to create a plugin and add it to a certain position in to the pipeline easily.

>**Note**:<br>
> To simulate upstream failure in the example, an assistant plugin is added to the pipeline, you need not it in the real case.

### Plugin

1. [HTTP input](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#http-input-plugin): To enable HTTPS endpoint to receive Ease Monitor data send from the client.
2. [Service circuit breaker](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#service-circuit-breaker-plugin): Limiting request rate base on the failure rate the pass probability plugin simulated.
3. [IO reader](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#io-reader-plugin): To read Ease Monitor data from the client via HTTPS transport layer in to local memory for handling in next steps.
4. [Json validator](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#json-validator-plugin): Validating the Ease Monitor data send from the client is using a certain schema. You can use [Ease Monitor graphite validator](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#ease-monitor-graphite-validator-plugin) if you would like to use the pipeline to handle Ease Monitor data with graphite plaintext protocol.
5. [Ease Monitor Json GID extractor](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#ease-monitor-json-gid-extractor-plugin): Extracts Ease Monitor global ID from the Ease Monitor data. You can use [Ease Monitor graphite GID extractor](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#ease-monitor-graphite-gid-extractor-plugin) if you would like to use the pipeline to handle Ease Monitor data with graphite plaintext protocol.
6. [Static pass probability limiter](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#static-pass-probability-limiter-plugin): The plugin passes the reqeust with a fixed probability, in this case it is used to simulate upstream service failure with 50% probability.
7. [Kafka output](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#kafka-output-plugin): Sending the data to configured kafka topic, Ease Monitor pipeline will fetch them for rest of processes.

Using follow [Administration API](https://github.com/hexdecteam/easegateway/blob/master/doc/admin_api_ref.swagger.yaml) calls to setup above plugins:

```
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPInput", "config": {"plugin_name": "test-httpinput", "url": "/test", "method": "POST", "headers_enum": {"name": ["bar", "bar1"]}, "request_body_io_key": "HTTP_REQUEST_BODY_IO"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "ServiceCircuitBreaker", "config": {"plugin_name": "test-servicecircuitbreaker", "plugins_concerned": ["test-staticprobabilitylimiter"], "all_tps_threshold_to_enable": 1, "failure_tps_threshold_to_break": 1, "failure_tps_percent_threshold_to_break": -1, "recovery_time_msec": 500, "success_tps_threshold_to_open": 1}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "IOReader", "config": {"plugin_name": "test-ioreader", "input_key":"HTTP_REQUEST_BODY_IO", "output_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "JSONValidator", "config": {"plugin_name": "test-jsonvalidator", "schema": "{\"title\": \"Record\",\"type\": \"object\",\"properties\": {\"name\": {\"type\": \"string\"}}, \"required\": [\"name\"]}", "data_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "EaseMonitorJSONGidExtractor", "config": {"plugin_name": "test-jsongidextractor", "gid_key": "GID", "data_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "StaticProbabilityLimiter", "config": {"plugin_name": "test-staticprobabilitylimiter", "pass_pr": 0.5}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "KafkaOutput", "config": {"plugin_name": "test-kafkaoutput", "topic": "test", "brokers": ["192.168.98.130:9092"], "message_key_key": "GID", "data_key": "DATA"}}'
```

### Pipeline

We need to set the limiter plugin to a certain position in the pipeline, generally we should terminate request as earlier as possible in the pipeline since once breaking is happened there is no reason to handle the request for rest steps, in this case we add the limiter close to HTTP input plugin.

You can use follow [Administration API](https://github.com/hexdecteam/easegateway/blob/master/doc/admin_api_ref.swagger.yaml) calls to setup the pipeline:

```
$ curl http://127.0.0.1:9090/admin/v1/pipelines -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "LinearPipeline", "config": {"pipeline_name": "test-jsonpipeline", "plugin_names": ["test-httpinput", "test-servicecircuitbreaker", "test-ioreader", "test-jsonvalidator", "test-jsongidextractor", "test-staticprobabilitylimiter", "test-kafkaoutput"], "parallelism": 10}}'
```

### Test

```
$ ab -n 1000 -c 20 -H "name:bar" -T "application/json" -p ~/load -f TSL1.2 https://127.0.0.1:10443/test
This is ApacheBench, Version 2.3 <$Revision: 1528965 $>
Copyright 1996 Adam Twiss, Zeus Technology Ltd, http://www.zeustech.net/
Licensed to The Apache Software Foundation, http://www.apache.org/

Benchmarking 127.0.0.1 (be patient)
Completed 100 requests
Completed 200 requests
Completed 300 requests
Completed 400 requests
Completed 500 requests
Completed 600 requests
Completed 700 requests
Completed 800 requests
Completed 900 requests
Completed 1000 requests
Finished 1000 requests


Server Software:
Server Hostname:        127.0.0.1
Server Port:            10443
SSL/TLS Protocol:       TLSv1.2,ECDHE-RSA-AES128-GCM-SHA256,2048,128

Document Path:          /test
Document Length:        0 bytes

Concurrency Level:      20
Time taken for tests:   8.208 seconds
Complete requests:      1000
Failed requests:        0
Non-2xx responses:      560
Total transferred:      105400 bytes
Total body sent:        337000
HTML transferred:       0 bytes
Requests per second:    121.84 [#/sec] (mean)
Time per request:       164.151 [ms] (mean)
Time per request:       8.208 [ms] (mean, across all concurrent requests)
Transfer rate:          12.54 [Kbytes/sec] received
                        40.10 kb/s sent
                        52.64 kb/s total

Connection Times (ms)
              min  mean[+/-sd] median   max
Connect:        4   91  46.4     94     238
Processing:     1   71  51.0     62     350
Waiting:        1   22  48.1      2     339
Total:          6  162  54.3    160     383

Percentage of the requests served within a certain time (ms)
  50%    160
  66%    173
  75%    188
  80%    196
  90%    228
  95%    262
  98%    289
  99%    323
 100%    383 (longest request)
```

With updating ``recovery_time_msec`` option (it equals to [MTTR](https://en.wikipedia.org/wiki/Mean_time_to_recovery) generally) from 500 milliseconds to 5 seconds, we can see service circuit breaker can block request on failed upstream service efficiently.

```
$ curl http://127.0.0.1:9090/admin/v1/plugins -X PUT -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "ServiceCircuitBreaker", "config": {"plugin_name": "test-servicecircuitbreaker", "plugins_concerned": ["test-staticprobabilitylimiter"], "all_tps_threshold_to_enable": 1, "failure_tps_threshold_to_break": 1, "failure_tps_percent_threshold_to_break": -1, "recovery_time_msec": 5000, "success_tps_threshold_to_open": 1}}'
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8
X-Powered-By: go-json-rest
Date: Fri, 31 Mar 2017 10:13:36 GMT
Content-Length: 0

$ ab -n 1000 -c 20 -H "name:bar" -T "application/json" -p ~/load -f TSL1.2 https://127.0.0.1:10443/test
This is ApacheBench, Version 2.3 <$Revision: 1528965 $>
Copyright 1996 Adam Twiss, Zeus Technology Ltd, http://www.zeustech.net/
Licensed to The Apache Software Foundation, http://www.apache.org/

Benchmarking 127.0.0.1 (be patient)
Completed 100 requests
Completed 200 requests
Completed 300 requests
Completed 400 requests
Completed 500 requests
Completed 600 requests
Completed 700 requests
Completed 800 requests
Completed 900 requests
Completed 1000 requests
Finished 1000 requests


Server Software:
Server Hostname:        127.0.0.1
Server Port:            10443
SSL/TLS Protocol:       TLSv1.2,ECDHE-RSA-AES128-GCM-SHA256,2048,128

Document Path:          /test
Document Length:        0 bytes

Concurrency Level:      20
Time taken for tests:   5.988 seconds
Complete requests:      1000
Failed requests:        0
Non-2xx responses:      923
Total transferred:      110845 bytes
Total body sent:        337000
HTML transferred:       0 bytes
Requests per second:    167.00 [#/sec] (mean)
Time per request:       119.764 [ms] (mean)
Time per request:       5.988 [ms] (mean, across all concurrent requests)
Transfer rate:          18.08 [Kbytes/sec] received
                        54.96 kb/s sent
                        73.04 kb/s total

Connection Times (ms)
              min  mean[+/-sd] median   max
Connect:        4  108  31.0    118     144
Processing:     1   10  21.5      2     147
Waiting:        1    3  12.9      1     146
Total:          5  118  31.5    121     274

Percentage of the requests served within a certain time (ms)
  50%    121
  66%    124
  75%    128
  80%    132
  90%    138
  95%    154
  98%    187
  99%    200
 100%    274 (longest request)
```

## HTTP proxy with caching

In this case, you can see how a pipleine act a HTTP/HTTPS proxy and how to add a cache layer between input and upstream. The cache function is used to improve the performance of RESTful service automatically.

### Plugin

1. [HTTP input](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#http-input-plugin): To enable HTTP endpoint to receive RESTful request for upstream service.
2. [Simple common cache](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#simple-common-cache-plugin): To cache HTTP body upstream service responded. The body buffer will be cached in 10 seconds in this case. During the TTL (Time-To-Live) any request which hits the cache will renew the expiration time of the body buffer automatically.
3. [Simple common cache](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#simple-common-cache-plugin): To cache HTTP status code upstream service responded. The status code will be cached in 10 seconds in this case. Like body buffer, during the TTL any request which hits the cache will renew the expiration time of the status code as well.
4. [IO reader](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#io-reader-plugin): To read received data from the client via HTTP transport layer in to local memory as a proxy.
5. [HTTP output](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#http-output-plugin): Sending the body and headers to a certain endpoint of upstream RESTFul service.
6. [IO reader](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#io-reader-plugin): To read response data from the upstream service via HTTP transport layer in to local memory, the body buffer can be cached and will be responded to client in anyway.

Using follow [Administration API](https://github.com/hexdecteam/easegateway/blob/master/doc/admin_api_ref.swagger.yaml) calls to setup above plugins:

```
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPInput", "config": {"plugin_name": "test-httpinput", "url": "/test", "method": "GET", "headers_enum": {"name": ["bar", "bar1"]}, "request_body_io_key": "HTTP_REQUEST_BODY_IO", "response_code_key": "response_code", "response_body_buffer_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "SimpleCommonCache", "config": {"plugin_name": "test-simplecommoncache-body", "hit_keys": ["HTTP_NAME"], "cache_key": "DATA", "ttl_sec": 10, "finish_if_hit": false}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "SimpleCommonCache", "config": {"plugin_name": "test-simplecommoncache-code", "hit_keys": ["HTTP_NAME"], "cache_key": "response_code", "ttl_sec": 10, "finish_if_hit": true}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "IOReader", "config": {"plugin_name": "test-ioreader1", "input_key":"HTTP_REQUEST_BODY_IO", "output_key": "REQ_DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPOutput", "config": {"plugin_name": "test-httpoutput", "url_pattern": "http://127.0.0.1:1122/abc{def}/{ghi}", "header_patterns": {}, "method": "GET", "response_code_key": "response_code", "response_body_io_key": "HTTP_RESP_BODY_IO", "body_pattern": "{REQ_DATA}"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "IOReader", "config": {"plugin_name": "test-ioreader2", "input_key":"HTTP_RESP_BODY_IO", "output_key": "DATA"}}'
```

### Pipeline

We need to set both cache plugins to the certain position in the pipeline, in this case we would like to response client from cache as soon as possible since once cache is hit there is no reason to handle the request for rest steps in the pipeline, so we add the cache sutff close to HTTP input plugin.

You can use follow [Administration API](https://github.com/hexdecteam/easegateway/blob/master/doc/admin_api_ref.swagger.yaml) calls to setup the pipeline:

```
$ curl http://127.0.0.1:9090/admin/v1/pipelines -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "LinearPipeline", "config": {"pipeline_name": "test-jsonpipeline", "plugin_names": ["test-httpinput", "test-simplecommoncache-body", "test-simplecommoncache-code", "test-ioreader1", "test-httpoutput", "test-ioreader2"], "parallelism": 10}}'
```

### Test

A fake HTTP Server to output request log for demo.

```
$ cat ~/server.py
from BaseHTTPServer import BaseHTTPRequestHandler,HTTPServer

def main():
    try:
        port = 1122
        server = HTTPServer(('127.0.0.1', port), BaseHTTPRequestHandler)
        print "Web Server running on port %s" % port
        server.serve_forever()
    except KeyboardInterrupt:
        print " ^C entered, stopping web server...."
        server.socket.close()

main()

$ python ~/server.py
Web Server running on port 1122
127.0.0.1 - - [31/Mar/2017 22:56:37] code 501, message Unsupported method ('GET')
127.0.0.1 - - [31/Mar/2017 22:56:37] "GET /abc/ HTTP/1.1" 501 -
127.0.0.1 - - [31/Mar/2017 22:56:56] code 501, message Unsupported method ('GET')
127.0.0.1 - - [31/Mar/2017 22:56:56] "GET /abc/ HTTP/1.1" 501 -
^C ^C entered, stopping web server....
```

Sending out client requests to the proxy endpoint we created by above commands. You might check the timestamp in fake server and ``curl`` outputs.

```
$ curl -i -k https://127.0.0.1:10443/test -X GET -i -w "\n" -H "name:bar" -d "$LOAD"
HTTP/1.1 501 Not Implemented
Date: Fri, 31 Mar 2017 14:56:37 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<head>
<title>Error response</title>
</head>
<body>
<h1>Error response</h1>
<p>Error code 501.
<p>Message: Unsupported method ('GET').
<p>Error code explanation: 501 = Server does not support this operation.
</body>

$ curl -i -k https://127.0.0.1:10443/test -X GET -i -w "\n" -H "name:bar" -d "$LOAD"
HTTP/1.1 501 Not Implemented
Date: Fri, 31 Mar 2017 14:56:39 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<head>
<title>Error response</title>
</head>
<body>
<h1>Error response</h1>
<p>Error code 501.
<p>Message: Unsupported method ('GET').
<p>Error code explanation: 501 = Server does not support this operation.
</body>

$ curl -i -k https://127.0.0.1:10443/test -X GET -i -w "\n" -H "name:bar" -d "$LOAD"
HTTP/1.1 501 Not Implemented
Date: Fri, 31 Mar 2017 14:56:56 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<head>
<title>Error response</title>
</head>
<body>
<h1>Error response</h1>
<p>Error code 501.
<p>Message: Unsupported method ('GET').
<p>Error code explanation: 501 = Server does not support this operation.
</body>

$ curl -i -k https://127.0.0.1:10443/test -X GET -i -w "\n" -H "name:bar" -d "$LOAD"
HTTP/1.1 501 Not Implemented
Date: Fri, 31 Mar 2017 14:57:02 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<head>
<title>Error response</title>
</head>
<body>
<h1>Error response</h1>
<p>Error code 501.
<p>Message: Unsupported method ('GET').
<p>Error code explanation: 501 = Server does not support this operation.
```

## Service downgrading to protect critical service

In this case, we would like to show a way to make failure upstream service to returns mock data instead of exposing failure to the client direclty. In general downstream on client side is a critical service, it means even which depends on the upstream however the business in the service is not important that can be skipped if some issues happened at there, like loading customer comments for a commodity display page.

>**Note**:<br>
> To simulate upstream failure in the example, an assistant plugin is added to the pipeline, you need not it in the real case.

### Plugins

1. [HTTP input](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#http-input-plugin): To enable HTTPS endpoint to receive Ease Monitor data send from the client.
2. [Simple common mock](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#simple-common-mock-plugin): To returns mock data for the failure Ease Monitor service at upstream.
3. [Static pass probability limiter](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#static-pass-probability-limiter-plugin): The plugin passes the reqeust with a fixed probability, in this case it is used to simulate upstream service failure with 50% probability.
4. [IO reader](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#io-reader-plugin): To read Ease Monitor data from the client via HTTPS transport layer in to local memory for handling in next steps.
5. [Json validator](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#json-validator-plugin): Validating the Ease Monitor data send from the client is using a certain schema. You can use [Ease Monitor graphite validator](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#ease-monitor-graphite-validator-plugin) if you would like to use the pipeline to handle Ease Monitor data with graphite plaintext protocol.
6. [Ease Monitor Json GID extractor](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#ease-monitor-json-gid-extractor-plugin): Extracts Ease Monitor global ID from the Ease Monitor data. You can use [Ease Monitor graphite GID extractor](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#ease-monitor-graphite-gid-extractor-plugin) if you would like to use the pipeline to handle Ease Monitor data with graphite plaintext protocol.
7. [Kafka output](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#kafka-output-plugin): Sending the data to configured kafka topic, Ease Monitor pipeline will fetch them for rest of processes.

Using follow [Administration API](https://github.com/hexdecteam/easegateway/blob/master/doc/admin_api_ref.swagger.yaml) calls to setup above plugins:

```
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPInput", "config": {"plugin_name": "test-httpinput", "url": "/test", "method": "POST", "headers_enum": {"name": ["bar", "bar1"]}, "request_body_io_key": "HTTP_REQUEST_BODY_IO"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "SimpleCommonMock", "config": {"plugin_name": "test-simplecommonmock", "plugin_concerned": "test-staticprobabilitylimiter", "task_error_code_concerned": "ResultFlowControl", "mock_task_data_key": "example", "mock_task_data_value": "fake"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "StaticProbabilityLimiter", "config": {"plugin_name": "test-staticprobabilitylimiter", "pass_pr": 0.5}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "IOReader", "config": {"plugin_name": "test-ioreader", "input_key":"HTTP_REQUEST_BODY_IO", "output_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "JSONValidator", "config": {"plugin_name": "test-jsonvalidator", "schema": "{\"title\": \"Record\",\"type\": \"object\",\"properties\": {\"name\": {\"type\": \"string\"}}, \"required\": [\"name\"]}", "data_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "EaseMonitorJSONGidExtractor", "config": {"plugin_name": "test-jsongidextractor", "gid_key": "GID", "data_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "KafkaOutput", "config": {"plugin_name": "test-kafkaoutput", "topic": "test", "brokers": ["192.168.98.130:9092"], "message_key_key": "GID", "data_key": "DATA"}}'
```

### Pipeline

You can use follow [Administration API](https://github.com/hexdecteam/easegateway/blob/master/doc/admin_api_ref.swagger.yaml) calls to setup the pipeline:

```
$ curl http://127.0.0.1:9090/admin/v1/pipelines -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "LinearPipeline", "config": {"pipeline_name": "test-jsonpipeline", "plugin_names": ["test-httpinput", "test-simplecommonmock", "test-staticprobabilitylimiter", "test-ioreader", "test-jsonvalidator", "test-jsongidextractor", "test-kafkaoutput"], "parallelism": 10}}'
```

### Test

In output of ApacheBench you can see there is no any Non-2xx responses.

```
$ ab -n 100 -c 20 -H "name:bar" -T "application/json" -p ~/load -f TSL1.2 https://127.0.0.1:10443/test
This is ApacheBench, Version 2.3 <$Revision: 1528965 $>
Copyright 1996 Adam Twiss, Zeus Technology Ltd, http://www.zeustech.net/
Licensed to The Apache Software Foundation, http://www.apache.org/

Benchmarking 127.0.0.1 (be patient).....done


Server Software:
Server Hostname:        127.0.0.1
Server Port:            10443
SSL/TLS Protocol:       TLSv1.2,ECDHE-RSA-AES128-GCM-SHA256,2048,128

Document Path:          /test
Document Length:        0 bytes

Concurrency Level:      20
Time taken for tests:   0.561 seconds
Complete requests:      100
Failed requests:        0
Total transferred:      9700 bytes
Total body sent:        33700
HTML transferred:       0 bytes
Requests per second:    178.36 [#/sec] (mean)
Time per request:       112.136 [ms] (mean)
Time per request:       5.607 [ms] (mean, across all concurrent requests)
Transfer rate:          16.90 [Kbytes/sec] received
                        58.70 kb/s sent
                        75.59 kb/s total

Connection Times (ms)
              min  mean[+/-sd] median   max
Connect:        5   44  35.1     36     156
Processing:    11   67  21.7     71     115
Waiting:       11   65  21.6     68     115
Total:         52  111  42.6    114     216

Percentage of the requests served within a certain time (ms)
  50%    114
  66%    116
  75%    120
  80%    137
  90%    201
  95%    209
  98%    214
  99%    216
 100%    216 (longest request)
```

## Flash sale event support

In a flash sale event of e-Commerence case, the behaviors between user and website are like these:

1. Before the flash sale event starts, many users open the event page of the website and prepare to click the order link. At the moment the order link is under disable status since event is not started.
2. After the flash sale event starts, users click the order link and a large number of the order requests are sent to the website, and website handles the requests as much as posible on the basis of the service availability.
3. Once the commodity sold out, any incoming order request will be rejected.

So the biggest challenges to support this kind of event for a website backend is how to support massive order requests in a very short period and keep best availability of all related services. To handle the challenge, Ease Gateway provides 4 steps to cover above three cases separately and technologically:

1. Ease gateway can be distributed to have multiple instances to serve users come from different region or logical group. Under this way, we can increase the capacity of access layer by scaling out gateway instance easily, we are even preconfigure the capacity before the event.
2. Before the flash sale event starts, according to the user behavior gateway can provides a statistics data to indicate how many user are prepared and ready to order. And base on this statistics indicator, website can calculate and pre-configure the probability of success buying for different access endpoint on each gateway instance.
3. When the flash sale event starts, the gateway instance will reduce the massive order requests according to the pre-configured probability, and finally a reasonable volume of order requests hit the website backend service really. 
4. Once the website backend service responds gateway there is no more commodity in the stock, gateway returns user a standard error and reject all incoming order requests directly and no further requests hit the website.

To achieve above solution we need to prepare two dedicated pipelines for each gateway instance, first one is used to cover above point #2, and another one is used to cover point #3 and #4.

1. User session counting.
2. Rejecting requests based on the probability and completely rejecting once upstream returns a special failure.

### User session counting

#### Plugin

1. [HTTP input](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#http-input-plugin): To enable HTTP endpoint to receive user request for accessing flash sale event page.
2. [IO reader](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#io-reader-plugin): To read received data from the client via HTTP transport layer in to local memory as a proxy.
3. [HTTP output](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#http-output-plugin): Sending the body and headers to the flash sale event page of the website.
4. [HTTP header counter](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#http-header-counter-plugin): To calculate amount of different HTTP header value of a certain header name, and the amount will be deducted automatically if user has not any active with website more then 1 minute. The result is exposed by the statistics indicator ``RECENT_HEADER_COUNT``.

Using follow [Administration API](https://github.com/hexdecteam/easegateway/blob/master/doc/admin_api_ref.swagger.yaml) calls to setup above plugins:

```
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPInput", "config": {"plugin_name": "test-httpinput", "url": "/test", "method": "GET", "headers_enum": {"name": ["bar", "bar1"]}, "request_body_io_key": "HTTP_REQUEST_BODY_IO", "response_code_key": "response_code", "response_body_io_key": "HTTP_RESP_BODY_IO"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "IOReader", "config": {"plugin_name": "test-ioreader", "input_key":"HTTP_REQUEST_BODY_IO", "output_key": "REQ_DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPOutput", "config": {"plugin_name": "test-httpoutput", "url_pattern": "http://127.0.0.1:1122/abc{def}/{ghi}", "header_patterns": {}, "method": "GET", "response_code_key": "response_code", "response_body_io_key": "HTTP_RESP_BODY_IO", "body_pattern": "{REQ_DATA}"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPHeaderCounter", "config": {"plugin_name": "test-httpheadercounter", "header_concerned": "name", "expiration_min": 1}}'
```

#### Pipeline

Technically there is no requirement on the position of HTTP header counter plugin.

You can use follow [Administration API](https://github.com/hexdecteam/easegateway/blob/master/doc/admin_api_ref.swagger.yaml) calls to setup the pipeline:

```
curl http://127.0.0.1:9090/admin/v1/pipelines -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "LinearPipeline", "config": {"pipeline_name": "test-jsonpipeline", "plugin_names": ["test-httpinput", "test-ioreader", "test-httpoutput", "test-httpheadercounter"], "parallelism": 10}}'
```

#### Test

```
$ curl http://127.0.0.1:9090/statistics/v1/pipelines/test-jsonpipeline/plugins/test-httpheadercounter/indicators/RECENT_HEADER_COUNT/desc  -X GET -w "\n"
"The count of http requests that the header of each one contains a key 'name' in last 60 second(s)."

$ curl http://127.0.0.1:9090/statistics/v1/pipelines/test-jsonpipeline/plugins/test-httpheadercounter/indicators/RECENT_HEADER_COUNT/value  -X GET -w "\n"
0

$ curl -i -k https://127.0.0.1:10443/test -X GET -i -w "\n" -H "name:bar" -d "$LOAD"
HTTP/1.1 200 OK
Date: Fri, 31 Mar 2017 17:39:48 GMT
Content-Type: text/html; charset=utf-8
Content-Length: 0

$ curl http://127.0.0.1:9090/statistics/v1/pipelines/test-jsonpipeline/plugins/test-httpheadercounter/indicators/RECENT_HEADER_COUNT/value  -X GET -w "\n"
1

$ curl -i -k https://127.0.0.1:10443/test -X GET -i -w "\n" -H "name:bar1" -d "$LOAD"
HTTP/1.1 200 OK
Date: Fri, 31 Mar 2017 17:40:02 GMT
Content-Type: text/html; charset=utf-8
Content-Length: 0

$ curl http://127.0.0.1:9090/statistics/v1/pipelines/test-jsonpipeline/plugins/test-httpheadercounter/indicators/RECENT_HEADER_COUNT/value  -X GET -w "\n"
2
```

### Rejecting requests

#### Plugin

1. [HTTP input](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#http-input-plugin): To enable HTTP endpoint to receive user request for order commodity.
2. [No more failure limiter](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#no-more-failure-limiter-plugin): To returns a standard error and reject all incoming requests directly and no further requests hit the upstream.
3. [Static pass probability limiter](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#static-pass-probability-limiter-plugin): The plugin passes the reqeust with a fixed probability. In this case it is used to reduce the massive order requests and finally a reasonable volume of order requests hit the website backend service really.
4. [IO reader](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#io-reader-plugin): To read received data from the client via HTTP transport layer in to local memory as a proxy.
5. [HTTP output](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#http-output-plugin): Sending the body and headers to the order serivce of the website.

```
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPInput", "config": {"plugin_name": "test-httpinput", "url": "/test", "method": "GET", "headers_enum": {"name": ["bar", "bar1"]}, "request_body_io_key": "HTTP_REQUEST_BODY_IO", "response_code_key": "response_code", "response_body_io_key": "HTTP_RESP_BODY_IO"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "NoMoreFailureLimiter", "config": {"plugin_name": "test-nomorefailurelimiter", "failure_count_threshold": 1, "failure_task_data_key": "response_code", "failure_task_data_value": "400"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "StaticProbabilityLimiter", "config": {"plugin_name": "test-staticprobabilitylimiter", "pass_pr": 0.75}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "IOReader", "config": {"plugin_name": "test-ioreader", "input_key":"HTTP_REQUEST_BODY_IO", "output_key": "REQ_DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPOutput", "config": {"plugin_name": "test-httpoutput", "url_pattern": "http://127.0.0.1:1122/abc", "header_patterns": {}, "method": "GET", "response_code_key": "response_code", "response_body_io_key": "HTTP_RESP_BODY_IO", "body_pattern": "{REQ_DATA}"}}'
```

#### Pipeline

You can use follow [Administration API](https://github.com/hexdecteam/easegateway/blob/master/doc/admin_api_ref.swagger.yaml) calls to setup the pipeline:

```
$ curl http://127.0.0.1:9090/admin/v1/pipelines -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "LinearPipeline", "config": {"pipeline_name": "test-jsonpipeline", "plugin_names": ["test-httpinput", "test-nomorefailurelimiter", "test-staticprobabilitylimiter", "test-ioreader", "test-httpoutput"], "parallelism": 10}}'
```

#### Test

```
$ cat ~/server1.py
from BaseHTTPServer import BaseHTTPRequestHandler,HTTPServer

global stock
stock = 6

class WebServerHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        global stock
        if self.path.endswith("/abc"):
            stock -= 1
            if stock > 0:
                self.send_response(200)
                self.send_header('Content-type', 'text/html')
                self.end_headers()
                message = "<html><body>Order successfully!</body></html>"
                self.wfile.write(message)
            else:
                self.send_response(400)
                self.send_header('Content-type', 'text/html')
                self.end_headers()
                message = "<html><body>Order failed!</body></html>"
                self.wfile.write(message)
            return

def main():
    try:
        port = 1122
        server = HTTPServer(('127.0.0.1', port), WebServerHandler)
        print "Web Server running on port %s" % port
        server.serve_forever()
    except KeyboardInterrupt:
        print " ^C entered, stopping web server...."
        server.socket.close()

main()

$ python ~/server1.py
Web Server running on port 1122
127.0.0.1 - - [01/Apr/2017 02:26:00] "GET /abc HTTP/1.1" 200 -
127.0.0.1 - - [01/Apr/2017 02:26:04] "GET /abc HTTP/1.1" 200 -
127.0.0.1 - - [01/Apr/2017 02:26:06] "GET /abc HTTP/1.1" 200 -
127.0.0.1 - - [01/Apr/2017 02:26:07] "GET /abc HTTP/1.1" 200 -
127.0.0.1 - - [01/Apr/2017 02:26:08] "GET /abc HTTP/1.1" 200 -
127.0.0.1 - - [01/Apr/2017 02:26:11] "GET /abc HTTP/1.1" 400 -
^C ^C entered, stopping web server....
```

```
$ curl -i -k https://127.0.0.1:10443/test -X GET -i -w "\n" -H "name:bar1" -d "$LOAD"
HTTP/1.1 200 OK
Date: Fri, 31 Mar 2017 18:26:00 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<html><body>Order successfully!</body></html>

$ curl -i -k https://127.0.0.1:10443/test -X GET -i -w "\n" -H "name:bar1" -d "$LOAD"
HTTP/1.1 429 Too Many Requests              <= This is limited by static probability limiter plugin
Date: Fri, 31 Mar 2017 18:26:01 GMT
Content-Type: text/plain; charset=utf-8
Transfer-Encoding: chunked

$ curl -i -k https://127.0.0.1:10443/test -X GET -i -w "\n" -H "name:bar1" -d "$LOAD"
HTTP/1.1 200 OK
Date: Fri, 31 Mar 2017 18:26:04 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<html><body>Order successfully!</body></html>

$ curl -i -k https://127.0.0.1:10443/test -X GET -i -w "\n" -H "name:bar1" -d "$LOAD"
HTTP/1.1 429 Too Many Requests              <= This is limited by static probability limiter plugin
Date: Fri, 31 Mar 2017 18:26:05 GMT
Content-Type: text/plain; charset=utf-8
Transfer-Encoding: chunked

$ curl -i -k https://127.0.0.1:10443/test -X GET -i -w "\n" -H "name:bar1" -d "$LOAD"
HTTP/1.1 200 OK
Date: Fri, 31 Mar 2017 18:26:06 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<html><body>Order successfully!</body></html>

$ curl -i -k https://127.0.0.1:10443/test -X GET -i -w "\n" -H "name:bar1" -d "$LOAD"
HTTP/1.1 429 Too Many Requests              <= This is limited by static probability limiter plugin
Date: Fri, 31 Mar 2017 18:26:07 GMT
Content-Type: text/plain; charset=utf-8
Transfer-Encoding: chunked

$ curl -i -k https://127.0.0.1:10443/test -X GET -i -w "\n" -H "name:bar1" -d "$LOAD"
HTTP/1.1 200 OK
Date: Fri, 31 Mar 2017 18:26:07 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<html><body>Order successfully!</body></html>

$ curl -i -k https://127.0.0.1:10443/test -X GET -i -w "\n" -H "name:bar1" -d "$LOAD"
HTTP/1.1 200 OK
Date: Fri, 31 Mar 2017 18:26:08 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<html><body>Order successfully!</body></html>

$ curl -i -k https://127.0.0.1:10443/test -X GET -i -w "\n" -H "name:bar1" -d "$LOAD"
HTTP/1.1 400 Bad Request
Date: Fri, 31 Mar 2017 18:26:11 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<html><body>Order failed!</body></html>

$ curl -i -k https://127.0.0.1:10443/test -X GET -i -w "\n" -H "name:bar1" -d "$LOAD"
HTTP/1.1 429 Too Many Requests              <= This is limited by no more failure limiter plugin
Date: Fri, 31 Mar 2017 18:26:12 GMT
Content-Type: text/plain; charset=utf-8
Transfer-Encoding: chunked

$ curl -i -k https://127.0.0.1:10443/test -X GET -i -w "\n" -H "name:bar1" -d "$LOAD"
HTTP/1.1 429 Too Many Requests              <= This is limited by no more failure limiter plugin
Date: Fri, 31 Mar 2017 18:26:13 GMT
Content-Type: text/plain; charset=utf-8
Transfer-Encoding: chunked

$ curl -i -k https://127.0.0.1:10443/test -X GET -i -w "\n" -H "name:bar1" -d "$LOAD"
HTTP/1.1 429 Too Many Requests              <= This is limited by no more failure limiter plugin
Date: Fri, 31 Mar 2017 18:26:14 GMT
Content-Type: text/plain; charset=utf-8
Transfer-Encoding: chunked
```
