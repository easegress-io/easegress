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
3. [Json validator](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#json-validator-plugin): Validating the Ease Monitor data send from the client is using a certain schema. You can use [Graphite validator](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#graphite-validator-plugin) if you would like to use the pipeline to handle Ease Monitor data with graphite plaintext protocol.
4. [Json GID extractor](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#json-gid-extractor-plugin): Extractin Ease Monitor global ID from the Ease Monitor data. You can use [Graphite GID extractor](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#graphite-gid-extractor-plugin) if you would like to use the pipeline to handle Ease Monitor data with graphite plaintext protocol.
5. [Kafka output](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#kafka-output-plugin): Sending the data to configured kafka topic, Ease Monitor pipeline will fetch them for rest of processes.

Using follow [Administration API](https://github.com/hexdecteam/easegateway/blob/master/doc/admin_api_ref.swagger.yaml) calls to setup above plugins:

```
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPInput", "config": {"plugin_name": "test-httpinput", "url": "/test", "method": "POST", "headers_enum": {"name": ["bar", "bar1"]}, "request_body_io_key": "HTTP_REQUEST_BODY_IO"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "IOReader", "config": {"plugin_name": "test-ioreader", "input_key":"HTTP_REQUEST_BODY_IO", "output_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "JSONValidator", "config": {"plugin_name": "test-jsonvalidator", "schema": "{\"title\": \"Record\",\"type\": \"object\",\"properties\": {\"name\": {\"type\": \"string\"}}, \"required\": [\"name\"]}", "data_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "JSONGidExtractor", "config": {"plugin_name": "test-jsongidextractor", "gid_key": "GID", "data_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "KafkaOutput", "config": {"plugin_name": "test-kafkaoutput", "topic": "test", "brokers": ["192.168.98.130:9092"], "message_key_key": "GID", "data_key": "DATA"}}'
```

### Pipeline

You can use follow [Administration API](https://github.com/hexdecteam/easegateway/blob/master/doc/admin_api_ref.swagger.yaml) calls to setup above pipeline:

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
5. [Json GID extractor](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#json-gid-extractor-plugin): Extractin Ease Monitor global ID from the Ease Monitor data.
6. [Kafka output](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#kafka-output-plugin): Sending the data to configured kafka topic, Ease Monitor pipeline will fetch them for rest of processes.

Using follow [Administration API](https://github.com/hexdecteam/easegateway/blob/master/doc/admin_api_ref.swagger.yaml) calls to setup above plugins:

```
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPInput", "config": {"plugin_name": "test-httpinput", "url": "/test", "method": "POST", "headers_enum": {"name": ["bar", "bar1"]}, "request_body_io_key": "HTTP_REQUEST_BODY_IO"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "ThroughputRateLimiter", "config": {"plugin_name": "test-throughputratelimiter", "tps": "11"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "IOReader", "config": {"plugin_name": "test-ioreader", "input_key":"HTTP_REQUEST_BODY_IO", "output_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "JSONValidator", "config": {"plugin_name": "test-jsonvalidator", "schema": "{\"title\": \"Record\",\"type\": \"object\",\"properties\": {\"name\": {\"type\": \"string\"}}, \"required\": [\"name\"]}", "data_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "JSONGidExtractor", "config": {"plugin_name": "test-jsongidextractor", "gid_key": "GID", "data_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "KafkaOutput", "config": {"plugin_name": "test-kafkaoutput", "topic": "test", "brokers": ["192.168.98.130:9092"], "message_key_key": "GID", "data_key": "DATA"}}'
```

#### Pipeline

We need to set throughput reate limiter plugin to a certain position in the pipeline, generally we should terminate request as earlier as possible in the pipeline since once limiation is reached there is no reason to handle the request for rest steps, in this case we add the limiter close to HTTP input plugin.

You can use follow [Administration API](https://github.com/hexdecteam/easegateway/blob/master/doc/admin_api_ref.swagger.yaml) calls to setup above pipeline:

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
5. [Json GID extractor](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#json-gid-extractor-plugin): Extractin Ease Monitor global ID from the Ease Monitor data.
6. [Kafka output](https://github.com/hexdecteam/easegateway/blob/master/doc/plugin_ref.md#kafka-output-plugin): Sending the data to configured kafka topic, Ease Monitor pipeline will fetch them for rest of processes.

Using follow [Administration API](https://github.com/hexdecteam/easegateway/blob/master/doc/admin_api_ref.swagger.yaml) calls to setup above plugins:

```
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPInput", "config": {"plugin_name": "test-httpinput", "url": "/test", "method": "POST", "headers_enum": {"name": ["bar", "bar1"]}, "request_body_io_key": "HTTP_REQUEST_BODY_IO"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "LatencyWindowLimiter", "config": {"plugin_name": "test-latencywindowlimiter", "latency_threshold_msec": 150, "plugins_concerned": ["test-kafkaoutput"], "window_size_max": 10, "windows_size_init": 5}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "IOReader", "config": {"plugin_name": "test-ioreader", "input_key":"HTTP_REQUEST_BODY_IO", "output_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "JSONValidator", "config": {"plugin_name": "test-jsonvalidator", "schema": "{\"title\": \"Record\",\"type\": \"object\",\"properties\": {\"name\": {\"type\": \"string\"}}, \"required\": [\"name\"]}", "data_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "JSONGidExtractor", "config": {"plugin_name": "test-jsongidextractor", "gid_key": "GID", "data_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "KafkaOutput", "config": {"plugin_name": "test-kafkaoutput", "topic": "test", "brokers": ["192.168.98.130:9092"], "message_key_key": "GID", "data_key": "DATA"}}'
```

#### Pipeline

We need to set throughput reate limiter plugin to a certain position in the pipeline, generally we should terminate request as earlier as possible in the pipeline since once limiation is reached there is no reason to handle the request for rest steps, in this case we add the limiter close to HTTP input plugin.

You can use follow [Administration API](https://github.com/hexdecteam/easegateway/blob/master/doc/admin_api_ref.swagger.yaml) calls to setup above pipeline:

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
lzy@u1:~$ ab -n 100 -c 20 -H "name:bar" -T "application/json" -p ~/load -f TSL1.2 https://127.0.0.1:10443/test
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

## HTTP proxy with caching

## Service downgrading to protect critical service

## Flash sale event support