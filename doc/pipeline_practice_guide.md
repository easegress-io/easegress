# Ease Gateway Pipeline Practice Guide

## Overview

In this document, we would like to introduce follow pipelines from real practice to cover some worth use cases as examples.

| Name | Description | Complexity level |
|:--|:--|:--:|
| [Ease Monitor edge service](#ease-monitor-edge-service) | Runs an example HTTPS endpoint to receive an Ease Monitor data, processes it in the pipeline and sends prepared data to kafka finally. | Beginner |
| [HTTP traffic throttling](#http-traffic-throttling) | Performs latency and throughput rate based traffic control. | Beginner |
| [Service circuit breaking](#service-circuit-breaking) | As a protection function, once the service failures reach a certain threshold all further calls to the service will be returned with an error directly, and when the service recovery the breaking function will be disabled automatically. | Beginner |
| [HTTP streamy proxy](#http-streamy-proxy) | Works as a streamy HTTP/HTTPS proxy between client and upstream | Beginner |
| [HTTP proxy with load routing](#http-proxy-with-load-routing) | Works as a streamy HTTP/HTTPS proxy between client and upstream with a route selection policy. Blue/Green deployment, A/B testing and Retry are example use cases. | Intermediate |
| [HTTP proxy with caching](#http-proxy-with-caching) | Caches HTTP/HTTPS response for duplicated request | Intermediate |
| [Service downgrading to protect critical service](#service-downgrading-to-protect-critical-service) | Under unexpected traffic which higher than planed, sacrifice the unimportant services but keep critical request is handled. | Intermediate |
| [Flash sale event support](#flash-sale-event-support) | A pair of pipelines to support flash sale event. For e-Commerce, it means we have very low price items with limited stock, but have huge amount of people online compete on that. | Advanced |

> Kindly reminder: You could use the scripts at `src/inventory/script/utils` to simplify testing commands.

## Ease Monitor edge service

In this case, we will prepare a pipeline to runs necessary processes as Ease Monitor edge service endpoint.This a basic case for some of the following cases, so it's highly recommended to get through it.

### Plugin

1. [HTTP server](./plugin_ref.md#http-server-plugin): To enable HTTP server to listen on port 10080.
2. [HTTP input](./plugin_ref.md#http-input-plugin): To enable HTTPS endpoint to receive Ease Monitor data sent from the client.
3. [IO reader](./plugin_ref.md#io-reader-plugin): To read Ease Monitor data from the client via HTTPS transport layer in to local memory for handling in next steps.
4. [JSON validator](./plugin_ref.md#json-validator-plugin): Validating the Ease Monitor data sent from the client is using a certain schema. You can use [Ease Monitor graphite validator](./plugin_ref.md#ease-monitor-graphite-validator-plugin) if you would like to use the pipeline to handle Ease Monitor data with graphite plaintext protocol.
5. [Ease Monitor JSON GID extractor](./plugin_ref.md#ease-monitor-json-gid-extractor-plugin): Extracts Ease Monitor global ID from the Ease Monitor data. You can use [Ease Monitor graphite GID extractor](./plugin_ref.md#ease-monitor-graphite-gid-extractor-plugin) if you would like to use the pipeline to handle Ease Monitor data with graphite plaintext protocol.
6. [Kafka output](./plugin_ref.md#kafka-output-plugin): Sending the data to configured kafka topic, Ease Monitor pipeline will fetch them for rest of processes.

Using follow [Administration API](./admin_api_ref.swagger.yaml) calls to setup above plugins:

```
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPServer", "config": {"plugin_name": "httpserver", "port":10080, "keepalive_sec":30}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPInput", "config": {"plugin_name": "test-httpinput", "server_name": "httpserver", "path": "/test", "methods": ["POST"], "headers_enum": {"name": ["bar", "bar1"]}, "request_body_io_key": "HTTP_REQUEST_BODY_IO"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "IOReader", "config": {"plugin_name": "test-ioreader", "input_key": "HTTP_REQUEST_BODY_IO", "output_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "JSONValidator", "config": {"plugin_name": "test-jsonvalidator", "schema": "{\"title\": \"Record\",\"type\": \"object\",\"properties\": {\"name\": {\"type\": \"string\"}}, \"required\": [\"name\"]}", "data_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "EaseMonitorJSONGidExtractor", "config": {"plugin_name": "test-jsongidextractor", "gid_key": "GID", "data_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "KafkaOutput", "config": {"plugin_name": "test-kafkaoutput", "topic": "test", "brokers": ["127.0.0.1:9092"], "message_key_key": "GID", "data_key": "DATA"}}'
```

### Pipeline

You can use follow [Administration API](./admin_api_ref.swagger.yaml) calls to setup the pipeline:

```
$ curl http://127.0.0.1:9090/admin/v1/pipelines -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "LinearPipeline", "config": {"pipeline_name": "test-jsonpipeline", "plugin_names": ["httpserver", "test-httpinput", "test-ioreader", "test-jsonvalidator", "test-jsongidextractor", "test-kafkaoutput"], "parallelism": 10}}'
```

### Test

Once the pipeline is created (above Administration API returns HTTP 200), any incoming Ease Monitor data sent to the URL with certain method and header will be handled in pipeline and sent to the kafka topic. You might send test requests by blow commands. The data in `~/load` file just shows you an example, and you could refer Ease Monitor document to check complete data.

```
$ cat ~/load
{
        "name": "test-workload",
        "system": "ExampleSystem",
        "application": "ExampleApplication",
        "instance": "ExampleInstance",
        "hostname": "ExampleHost",
        "hostipv4": "127.0.0.1"
}
$ LOAD=`cat ~/load`
$ curl -i -k https://127.0.0.1:10443/test -X POST -i -w "\n" -H "name:bar" -d "$LOAD"
```

## HTTP traffic throttling

Currently Ease Gateway supports two kinds of traffic throttling:

* Throughput rate based throttling. This kind traffic throttling provides a clear and predictable workload limitation on the upstream, any exceeded requests will all be rejected by a ResultFlowControl internal error. Finally the error will be translated to a special response to the client, for example, HTTP Input plugin translates the error to HTTP status code 429 (StatusTooManyRequests, RFC 6585.4). There are two potential limitations on this traffic throttling way:
	* When upstream is deployed in a distributed environment, it is hard to setup a accurate throughput rate limitation according to a single service instance due to requests on the upstream could be passed by different instances.
	* When Ease Gateway is deployed in a distributed environment and more than one gateway instances are pointed to the same upstream service, under this case the requests could be handled by different gateway instances, so a clear throughput rate limitation in a single gateway instance does not really work on upstream.
* Upstream latency based throttling. This kind of traffic throttling provides a sliding window based on upstream handling latency, just like TCP flow control implementation the size of sliding window will be adjusted according to the case of upstream response, lower latency generally means better performance upstream has and more requests could be sent to upstream quickly. This traffic throttling way solved above two potential limitations of throughput rate based throttling method.

### Throughput rate based throttling

In this case, we will prepare a pipeline to show you how to setup a throughput rate limitation for Ease Monitor edge service. You can see we only need to create a plugin and add it to a certain position in to the pipeline easily.

#### Plugin

1. [HTTP server](./plugin_ref.md#http-server-plugin): To enable HTTP server to listen on port 10080.
2. [HTTP input](./plugin_ref.md#http-input-plugin): To enable HTTPS endpoint to receive Ease Monitor data sent from the client.
3. [Throughput rate limiter](./plugin_ref.md#throughput-rate-limiter-plugin): To add a limitation to control request rate. The limitation will be performed no matter how many concurrent clients send the request. In this case, we ask there are no more than 11 requests per second sent to Ease Monitor pipeline.
4. [IO reader](./plugin_ref.md#io-reader-plugin): To read Ease Monitor data from the client via HTTPS transport layer in to local memory for handling in next steps.
5. [JSON validator](./plugin_ref.md#json-validator-plugin): Validating the Ease Monitor data sent from the client is using a certain schema.
6. [Ease Monitor JSON GID extractor](./plugin_ref.md#ease-monitor-json-gid-extractor-plugin): Extracts Ease Monitor global ID from the Ease Monitor data. You can use [Graphite GID extractor](./plugin_ref.md#graphite-gid-extractor-plugin) if you would like to use the pipeline to handle Ease Monitor data with graphite plaintext protocol.
7. [Kafka output](./plugin_ref.md#kafka-output-plugin): Sending the data to configured kafka topic, Ease Monitor pipeline will fetch them for rest of processes.

Using follow [Administration API](./admin_api_ref.swagger.yaml) calls to setup above plugins:

```
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPServer", "config": {"plugin_name": "httpserver", "port":10080, "keepalive_sec":30}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPInput", "config": {"plugin_name": "test-httpinput", "server_name": "httpserver", "path": "/test", "methods": ["POST"], "headers_enum": {"name": ["bar", "bar1"]}, "request_body_io_key": "HTTP_REQUEST_BODY_IO"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "ThroughputRateLimiter", "config": {"plugin_name": "test-throughputratelimiter", "tps": "11"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "IOReader", "config": {"plugin_name": "test-ioreader", "input_key": "HTTP_REQUEST_BODY_IO", "output_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "JSONValidator", "config": {"plugin_name": "test-jsonvalidator", "schema": "{\"title\": \"Record\",\"type\": \"object\",\"properties\": {\"name\": {\"type\": \"string\"}}, \"required\": [\"name\"]}", "data_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "EaseMonitorJSONGidExtractor", "config": {"plugin_name": "test-jsongidextractor", "gid_key": "GID", "data_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "KafkaOutput", "config": {"plugin_name": "test-kafkaoutput", "topic": "test", "brokers": ["127.0.0.1:9092"], "message_key_key": "GID", "data_key": "DATA"}}'
```

#### Pipeline

We need to set throughput rate limiter plugin to a certain position in the pipeline, generally we should terminate request as earlier as possible in the pipeline since once limitation is reached there is no reason to handle the request for rest steps, in this case we add the limiter close to HTTP input plugin.

You can use follow [Administration API](./admin_api_ref.swagger.yaml) calls to setup the pipeline:

```
$ curl http://127.0.0.1:9090/admin/v1/pipelines -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "LinearPipeline", "config": {"pipeline_name": "test-jsonpipeline", "plugin_names": ["httpserver", "test-httpinput", "test-throughputratelimiter", "test-ioreader", "test-jsonvalidator", "test-jsongidextractor", "test-kafkaoutput"], "parallelism": 10}}'
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
        "hostipv4": "127.0.0.1"
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

1. In this case, we will prepare a pipeline to show you how to setup a latency based limitation for Ease Monitor edge service. You can see we only need to create a plugin and add it to a certain position in to the pipeline easily, it just like what we did in throughput rate based throttling above.

#### Plugin

1. [HTTP server](./plugin_ref.md#http-server-plugin): To enable HTTP server to listen on port 10080.
2. [HTTP input](./plugin_ref.md#http-input-plugin): To enable HTTPS endpoint to receive Ease Monitor data sent from the client.
3. [Latency limiter](./plugin_ref.md#latency-limiter-plugin): To add a limitation to control request rate. The limitation will be performed no matter how many concurrent clients send the request. In this case, we request to backoff follow requests 1 second if the process time of output kafka broker for previous request greater than 150 milliseconds.
4. [IO reader](./plugin_ref.md#io-reader-plugin): To read Ease Monitor data from the client via HTTPS transport layer in to local memory for handling in next steps.
5. [JSON validator](./plugin_ref.md#json-validator-plugin): Validating the Ease Monitor data sent from the client is using a certain schema.
6. [Ease Monitor JSON GID extractor](./plugin_ref.md#ease-monitor-json-gid-extractor-plugin): Extracts Ease Monitor global ID from the Ease Monitor data. You can use [Graphite GID extractor](./plugin_ref.md#graphite-gid-extractor-plugin) if you would like to use the pipeline to handle Ease Monitor data with graphite plaintext protocol.
7. [Kafka output](./plugin_ref.md#kafka-output-plugin): Sending the data to configured kafka topic, Ease Monitor pipeline will fetch them for rest of processes.

Using follow [Administration API](./admin_api_ref.swagger.yaml) calls to setup above plugins:

```
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPServer", "config": {"plugin_name": "httpserver", "port":10080, "keepalive_sec":30}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPInput", "config": {"plugin_name": "test-httpinput", "server_name": "httpserver", "path": "/test", "methods": ["POST"], "headers_enum": {"name": ["bar", "bar1"]}, "request_body_io_key": "HTTP_REQUEST_BODY_IO"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "LatencyWindowLimiter", "config": {"plugin_name": "test-latencywindowlimiter", "latency_threshold_msec": 150, "plugins_concerned": ["test-kafkaoutput"], "allow_times": 0, "backoff_msec": 1000}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "IOReader", "config": {"plugin_name": "test-ioreader", "input_key": "HTTP_REQUEST_BODY_IO", "output_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "JSONValidator", "config": {"plugin_name": "test-jsonvalidator", "schema": "{\"title\": \"Record\",\"type\": \"object\",\"properties\": {\"name\": {\"type\": \"string\"}}, \"required\": [\"name\"]}", "data_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "EaseMonitorJSONGidExtractor", "config": {"plugin_name": "test-jsongidextractor", "gid_key": "GID", "data_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "KafkaOutput", "config": {"plugin_name": "test-kafkaoutput", "topic": "test", "brokers": ["127.0.0.1:9092"], "message_key_key": "GID", "data_key": "DATA"}}'
```

#### Pipeline

We need to set the limiter plugin to a certain position in the pipeline, generally we should terminate request as earlier as possible in the pipeline since once limitation is reached there is no reason to handle the request for rest steps, in this case we add the limiter close to HTTP input plugin.

You can use follow [Administration API](./admin_api_ref.swagger.yaml) calls to setup the pipeline:

```
$ curl http://127.0.0.1:9090/admin/v1/pipelines -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "LinearPipeline", "config": {"pipeline_name": "test-jsonpipeline", "plugin_names": ["httpserver", "test-httpinput", "test-latencywindowlimiter", "test-ioreader", "test-jsonvalidator", "test-jsongidextractor", "test-kafkaoutput"], "parallelism": 10}}'
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

2. In follow case, we will prepare a pipeline to show you how to setup a latency based limitation for http proxy case. The idea is similar with above one. In this case, EaseGateway is acting a http proxy and a latency limiter is applied, we suggest to configure IO reader plugin in the pipeline, with the reader the execution time of receiving the upstream response body could be calculated by latency limiter, additionally, the execution time of HTTP output plugin will includes the socket initialization time and the time of receiving HTTP response header of the upstream.

#### Plugin

1. [HTTP server](./plugin_ref.md#http-server-plugin): To enable HTTP server to listen on port 10080.
2. [HTTP input](./plugin_ref.md#http-input-plugin): To enable HTTP endpoint to receive the data sent from the client.
3. [Latency limiter](./plugin_ref.md#latency-limiter-plugin): To add a limitation to control request rate. The limitation will be performed no matter how many concurrent clients send the request. In this case, we request to backoff follow requests 100 milliseconds if the process time of output kafka broker for previous request greater than 500 milliseconds.
4. [Upstream output](./plugin_ref.md#upstream-output-plugin): To output request to an upstream pipeline and waits the response.
5. [IO reader](./plugin_ref.md#io-reader-plugin): To read the data from the upstream via HTTP transport layer in to local memory for handling in next steps.
6. [Downstream input](./plugin_ref.md#downstream-input-plugin): Handles downstream request to running pipeline as input and send the response back.
7. [HTTP output](./plugin_ref.md#http-output-plugin): Sending the body and headers to a certain endpoint of upstream RESTFul service.

Using follow [Administration API](./admin_api_ref.swagger.yaml) calls to setup above plugins:

```
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPServer", "config": {"plugin_name": "httpserver", "port":10080, "keepalive_sec":30}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPInput", "config": {"plugin_name": "httpinput", "server_name": "httpserver", "dump_request": "auto", "fast_close":false, "headers_enum":null, "methods":["GET", "POST"], "request_body_io_key": "HTTP_REQUEST_BODY_IO", "request_header_names_key": "", "respond_error":false, "response_body_buffer_key": "HTTP_RESP_BODY_DATA", "response_body_io_key": "", "response_code_key": "HTTP_RESP_CODE", "unzip":true, "path": "\/test"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "LatencyLimiter", "config": {"plugin_name": "limiter", "backoff_timeout_msec":100, "latency_threshold_msec":500, "plugins_concerned":["upstreamoutput", "ioreader"], "allow_msec":0}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "UpstreamOutput", "config": {"plugin_name": "upstreamoutput", "target_pipelines":["upstream"], "route_policy": "round_robin", "timeout_sec":0, "request_data_keys":["HTTP_REQUEST_BODY_IO", "CONTENT_TYPE", "REQUEST_METHOD", "QUERY_STRING"], "target_weights":null, "value_hashed_keys":null, "filter_conditions":null, "target_response_flags":null}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "IOReader", "config": {"plugin_name": "ioreader", "input_key": "HTTP_RESP_BODY_IO", "output_key": "HTTP_RESP_BODY_DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "DownstreamInput", "config": {"plugin_name": "downstreaminput", "response_data_keys":["HTTP_RESP_CODE", "HTTP_RESP_BODY_IO"]}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPOutput", "config": {"plugin_name": "httpoutput", "ca_file": "", "cert_file": "", "close_body_after_pipeline":false, "dump_request": "auto", "dump_response": "auto", "expected_response_codes":[200, 500, 404, 405], "header_patterns": {"Content-Type": "{CONTENT_TYPE}"}, "insecure_tls":false, "keepalive": "auto", "keepalive_sec":30, "key_file": "", "method": "POST", "request_body_buffer_pattern": "", "request_body_io_key": "HTTP_REQUEST_BODY_IO", "response_body_io_key": "HTTP_RESP_BODY_IO", "response_code_key": "RESPONSE_CODE", "timeout_sec":120, "url_pattern": "http:\/\/127.0.0.1:1122\/abc"}}'
```

#### Pipeline

We need to set the limiter plugin to a certain position in the pipeline, generally we should terminate request as earlier as possible in the pipeline since once limitation is reached there is no reason to handle the request for rest steps, in this case we add the limiter close to HTTP input plugin.

You can use follow [Administration API](./admin_api_ref.swagger.yaml) calls to setup the pipelines:

```
$ curl http://127.0.0.1:9090/admin/v1/pipelines -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "LinearPipeline", "config": {"pipeline_name": "downstream", "plugin_names":["httpserver", "httpinput", "limiter", "upstreamoutput", "ioreader"], "parallelism":2, "cross_pipeline_request_backlog":0}}'
$ curl http://127.0.0.1:9090/admin/v1/pipelines -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "LinearPipeline", "config": {"cross_pipeline_request_backlog":0, "parallelism":2, "pipeline_name": "upstream", "plugin_names":["downstreaminput", "httpoutput"]}}'
```

#### Test

```
$ ab -c 2 -n 10 http://127.0.0.1:10080/test
This is ApacheBench, Version 2.3 <$Revision: 1528965 $>
Copyright 1996 Adam Twiss, Zeus Technology Ltd, http://www.zeustech.net/
Licensed to The Apache Software Foundation, http://www.apache.org/

Benchmarking 127.0.0.1 (be patient).....done


Server Software:
Server Hostname:        127.0.0.1
Server Port:            10080

Document Path:          /test
Document Length:        64 bytes

Concurrency Level:      2
Time taken for tests:   2.022 seconds
Complete requests:      10
Failed requests:        0
Total transferred:      1600 bytes
HTML transferred:       640 bytes
Requests per second:    4.95 [#/sec] (mean)
Time per request:       404.409 [ms] (mean)
Time per request:       202.204 [ms] (mean, across all concurrent requests)
Transfer rate:          0.77 [Kbytes/sec] received

Connection Times (ms)
              min  mean[+/-sd] median   max
Connect:        0    0   0.9      0       3
Processing:   205  384  63.0    403     406
Waiting:      202  383  63.7    403     406
Total:        205  384  62.9    403     407

Percentage of the requests served within a certain time (ms)
  50%    403
  66%    404
  75%    406
  80%    406
  90%    407
  95%    407
  98%    407
  99%    407
 100%    407 (longest request)
```

## Service circuit breaking

In this case, we will prepare a pipeline to show you how to add such a service circuit breaking mechanism to Ease Monitor edge service. You can see we only need to create a plugin and add it to a certain position in to the pipeline easily.

>**Note**:
> To simulate upstream failure in the example, an assistant plugin is added to the pipeline, you need not it in the real case.

### Plugin

1. [HTTP server](./plugin_ref.md#http-server-plugin): To enable HTTP server to listen on port 10080.
2. [HTTP input](./plugin_ref.md#http-input-plugin): To enable HTTPS endpoint to receive Ease Monitor data sent from the client.
3. [Service circuit breaker](./plugin_ref.md#service-circuit-breaker-plugin): Limiting request rate base on the failure rate the pass probability plugin simulated.
4. [IO reader](./plugin_ref.md#io-reader-plugin): To read Ease Monitor data from the client via HTTPS transport layer in to local memory for handling in next steps.
5. [JSON validator](./plugin_ref.md#json-validator-plugin): Validating the Ease Monitor data sent from the client is using a certain schema. You can use [Ease Monitor graphite validator](./plugin_ref.md#ease-monitor-graphite-validator-plugin) if you would like to use the pipeline to handle Ease Monitor data with graphite plaintext protocol.
6. [Ease Monitor JSON GID extractor](./plugin_ref.md#ease-monitor-json-gid-extractor-plugin): Extracts Ease Monitor global ID from the Ease Monitor data. You can use [Ease Monitor graphite GID extractor](./plugin_ref.md#ease-monitor-graphite-gid-extractor-plugin) if you would like to use the pipeline to handle Ease Monitor data with graphite plaintext protocol.
7. [Static pass probability limiter](./plugin_ref.md#static-pass-probability-limiter-plugin): The plugin passes the request with a fixed probability, in this case it is used to simulate upstream service failure with 50% probability.
8. [Kafka output](./plugin_ref.md#kafka-output-plugin): Sending the data to configured kafka topic, Ease Monitor pipeline will fetch them for rest of processes.

Using follow [Administration API](./admin_api_ref.swagger.yaml) calls to setup above plugins:

```
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPServer", "config": {"plugin_name": "httpserver", "port":10080, "keepalive_sec":30}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPInput", "config": {"plugin_name": "test-httpinput", "server_name": "httpserver", "path": "/test", "methods": ["POST"], "headers_enum": {"name": ["bar", "bar1"]}, "request_body_io_key": "HTTP_REQUEST_BODY_IO"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "ServiceCircuitBreaker", "config": {"plugin_name": "test-servicecircuitbreaker", "plugins_concerned": ["test-staticprobabilitylimiter"], "all_tps_threshold_to_enable": 1, "failure_tps_threshold_to_break": 1, "failure_tps_percent_threshold_to_break": -1, "recovery_time_msec": 500, "success_tps_threshold_to_open": 1}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "IOReader", "config": {"plugin_name": "test-ioreader", "input_key": "HTTP_REQUEST_BODY_IO", "output_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "JSONValidator", "config": {"plugin_name": "test-jsonvalidator", "schema": "{\"title\": \"Record\",\"type\": \"object\",\"properties\": {\"name\": {\"type\": \"string\"}}, \"required\": [\"name\"]}", "data_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "EaseMonitorJSONGidExtractor", "config": {"plugin_name": "test-jsongidextractor", "gid_key": "GID", "data_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "StaticProbabilityLimiter", "config": {"plugin_name": "test-staticprobabilitylimiter", "pass_pr": 0.5}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "KafkaOutput", "config": {"plugin_name": "test-kafkaoutput", "topic": "test", "brokers": ["127.0.0.1:9092"], "message_key_key": "GID", "data_key": "DATA"}}'
```

### Pipeline

We need to set the limiter plugin to a certain position in the pipeline, generally we should terminate request as earlier as possible in the pipeline since once breaking is happened there is no reason to handle the request for rest steps, in this case we add the limiter close to HTTP input plugin.

You can use follow [Administration API](./admin_api_ref.swagger.yaml) calls to setup the pipeline:

```
$ curl http://127.0.0.1:9090/admin/v1/pipelines -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "LinearPipeline", "config": {"pipeline_name": "test-jsonpipeline", "plugin_names": ["httpserver", "test-httpinput", "test-servicecircuitbreaker", "test-ioreader", "test-jsonvalidator", "test-jsongidextractor", "test-staticprobabilitylimiter", "test-kafkaoutput"], "parallelism": 10}}'
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

## HTTP streamy proxy

In this case, you can see how a pipeline act a HTTP/HTTPS proxy between input and upstream. There is no any buffering and unnecessary operations in the middle.

### Plugin

1. [HTTP server](./plugin_ref.md#http-server-plugin): To enable HTTP server to listen on port 10080.
2. [HTTP input](./plugin_ref.md#http-input-plugin): To enable HTTP endpoint to receive RESTful request for upstream service.
3. [HTTP output](./plugin_ref.md#http-output-plugin): Sending the body and headers to a certain endpoint of upstream RESTFul service.

Using follow [Administration API](./admin_api_ref.swagger.yaml) calls to setup above plugins:

```
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPServer", "config": {"plugin_name": "httpserver", "port":10080, "keepalive_sec":30}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPInput", "config": {"plugin_name": "test-httpinput", "server_name": "httpserver", "path": "/test", "methods": ["GET"], "headers_enum": {"name": ["bar", "bar1"]}, "request_body_io_key": "HTTP_REQUEST_BODY_IO", "response_code_key": "HTTP_RESP_CODE", "response_body_io_key": "HTTP_RESP_BODY_IO"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPOutput", "config": {"plugin_name": "test-httpoutput", "url_pattern": "http://127.0.0.1:1122/abc", "header_patterns": {}, "method": "POST", "response_code_key": "HTTP_RESP_CODE", "response_body_io_key": "HTTP_RESP_BODY_IO", "request_body_io_key": "HTTP_REQUEST_BODY_IO" }}'
```

### Pipeline

You can use follow [Administration API](./admin_api_ref.swagger.yaml) calls to setup the pipeline:

```
$ curl http://127.0.0.1:9090/admin/v1/pipelines -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "LinearPipeline", "config": {"pipeline_name": "test-jsonpipeline", "plugin_names": ["httpserver", "test-httpinput", "test-httpoutput"], "parallelism": 10}}'
```

### Test

A fake HTTP Server to output request log for demo.

```
$ cat ~/server2.py
from BaseHTTPServer import BaseHTTPRequestHandler,HTTPServer

class WebServerHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        if self.path.endswith("/abc"):
            content_len = int(self.headers.get('Content-Length', 0))
            post_body = self.rfile.read(content_len)
            print content_len, post_body
            self.send_response(200)
            self.send_header('Content-type', 'text/html')
            self.end_headers()
            message = "<html><body>OK</body></html>"
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

$ python ~/server2.py
Web Server running on port 1122
185 {
	"name": "test-workload",
	"system": "ExampleSystem",
	"application": "ExampleApplication",
	"instance": "ExampleInstance",
	"hostname": "ExampleHost",
	"hostipv4": "127.0.0.1"
}
127.0.0.1 - - [04/Apr/2017 15:22:14] "POST /abc HTTP/1.1" 200 -
```

Sending out client requests to the proxy endpoint we created by above commands.

```
$ curl -i -k https://127.0.0.1:10443/test -X GET -i -w "\n" -H "name:bar" -d "$LOAD"
HTTP/1.1 200 OK
Date: Tue, 04 Apr 2017 07:22:14 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<html><body>OK</body></html>
```

## HTTP proxy with load routing

In this case, you can see how three pipelines co-works together, downstream pipeline receives HTTP/HTTPS request and sends to one of two upstream pipelines selected by round_robin and weighted_round_robin policy. The upstream timeout case will be practiced as well.


### Plugin

1. [HTTP server](./plugin_ref.md#http-server-plugin): To enable HTTP server to listen on port 10080.
2. [HTTP input](./plugin_ref.md#http-input-plugin): To enable HTTP endpoint to receive RESTful request for upstream service.
3. [HTTP output](./plugin_ref.md#http-output-plugin): Sending the body and headers to a certain endpoint of upstream RESTFul service.
4. [Upstream output](./plugin_ref.md#upstream-output-plugin): To output request to an upstream pipeline and waits the response.
5. [Downstream input](./plugin_ref.md#downstream-input-plugin): Handles downstream request to running pipeline as input and send the response back.

Using follow [Administration API](./admin_api_ref.swagger.yaml) calls to setup above plugins:

```
# For upstream #1
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "DownstreamInput", "config": {"plugin_name": "test-downstreamintpu1", "response_data_keys": ["HTTP_RESP_CODE", "HTTP_RESP_BODY_IO"] }}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPOutput", "config": {"plugin_name": "test-httpoutput1", "url_pattern": "http://127.0.0.1:1122/abc", "header_patterns": {}, "method": "POST", "response_code_key": "HTTP_RESP_CODE", "response_body_io_key": "HTTP_RESP_BODY_IO", "request_body_io_key": "HTTP_REQUEST_BODY_IO", "close_body_after_pipeline": false}}'

# For upstream #2
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "DownstreamInput", "config": {"plugin_name": "test-downstreaminput2", "response_data_keys": ["HTTP_RESP_CODE", "HTTP_RESP_BODY_IO"] }}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPOutput", "config": {"plugin_name": "test-httpoutput2", "url_pattern": "http://127.0.0.1:3344/abc", "header_patterns": {}, "method": "POST", "response_code_key": "HTTP_RESP_CODE", "response_body_io_key": "HTTP_RESP_BODY_IO", "request_body_io_key": "HTTP_REQUEST_BODY_IO", "close_body_after_pipeline": false}}'

# For downstream, round_robin policy is used at this time
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPServer", "config": {"plugin_name": "httpserver", "port":10080, "keepalive_sec":30}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPInput", "config": {"plugin_name": "test-httpinput", "server_name": "httpserver", "path": "/test", "methods": ["GET"], "headers_enum": {"name": ["bar", "bar1"]}, "request_body_io_key": "HTTP_REQUEST_BODY_IO", "response_code_key": "HTTP_RESP_CODE", "response_body_io_key": "HTTP_RESP_BODY_IO"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "UpstreamOutput", "config": {"plugin_name": "test-upstreamoutput1", "target_pipelines": ["test-upstream1", "test-upstream2"], "request_data_keys": ["HTTP_REQUEST_BODY_IO"], "route_policy": "round_robin"}}'

```

### Pipeline

You can use follow [Administration API](./admin_api_ref.swagger.yaml) calls to setup the pipeline:

```
# For upstream #1
$ curl http://127.0.0.1:9090/admin/v1/pipelines -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "LinearPipeline", "config": {"pipeline_name": "test-upstream1", "plugin_names": ["test-downstreamintpu1", "test-httpoutput1"], "parallelism": 10}}'

# For upstream #2
$ curl http://127.0.0.1:9090/admin/v1/pipelines -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "LinearPipeline", "config": {"pipeline_name": "test-upstream2", "plugin_names": ["test-downstreaminput2", "test-httpoutput2"], "parallelism": 10}}'

# For downstream
$ curl http://127.0.0.1:9090/admin/v1/pipelines -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "LinearPipeline", "config": {"pipeline_name": "input", "plugin_names": ["httpserver", "test-httpinput", "test-upstreamoutput1"], "parallelism": 10}}'

```

### Test

A fake HTTP Server to serve the request for demo.

```
$ cat ~/server3.py
from BaseHTTPServer import BaseHTTPRequestHandler,HTTPServer
import sys

class WebServerHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        if self.path.endswith("/abc"):
            content_len = int(self.headers.get('Content-Length', 0))
            post_body = self.rfile.read(content_len)
            print content_len, post_body
            self.send_response(200)
            self.send_header('Content-type', 'text/html')
            self.end_headers()
            message = "<html><body>OK - %s</body></html>" % sys.argv[1]
            self.wfile.write(message)
            return

def main():
    try:
        port = int(sys.argv[1])
        server = HTTPServer(('127.0.0.1', port), WebServerHandler)
        print "Web Server running on port %s" % port
        server.serve_forever()
    except KeyboardInterrupt:
        print " ^C entered, stopping web server...."
        server.socket.close()

main()

$ python ~/server3.py 1122
$ python ~/server3.py 3344 # in a different terminal
```

Sending out client requests to the proxy endpoint we created by above commands.

```
$ curl -i -k http://127.0.0.1:10080/test -X GET -i -w "\n" -H "name:bar" -d "$LOAD"
HTTP/1.1 200 OK
Date: Mon, 12 Jun 2017 06:24:59 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<html><body>OK - 1122</body></html>
$ curl -i -k http://127.0.0.1:10080/test -X GET -i -w "\n" -H "name:bar" -d "$LOAD"
HTTP/1.1 200 OK
Date: Mon, 12 Jun 2017 06:25:00 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<html><body>OK - 3344</body></html>
$ curl -i -k http://127.0.0.1:10080/test -X GET -i -w "\n" -H "name:bar" -d "$LOAD"
HTTP/1.1 200 OK
Date: Mon, 12 Jun 2017 06:25:00 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<html><body>OK - 1122</body></html>
$ curl -i -k http://127.0.0.1:10080/test -X GET -i -w "\n" -H "name:bar" -d "$LOAD"
HTTP/1.1 200 OK
Date: Mon, 12 Jun 2017 06:25:01 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<html><body>OK - 3344</body></html>
$ curl -i -k http://127.0.0.1:10080/test -X GET -i -w "\n" -H "name:bar" -d "$LOAD"
HTTP/1.1 200 OK
Date: Mon, 12 Jun 2017 06:25:01 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<html><body>OK - 1122</body></html>
$ curl -i -k http://127.0.0.1:10080/test -X GET -i -w "\n" -H "name:bar" -d "$LOAD"
HTTP/1.1 200 OK
Date: Mon, 12 Jun 2017 06:25:02 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<html><body>OK - 3344</body></html>
```

Let's take a look on the result with `weighted_round_robin` policy.

```
$ curl http://127.0.0.1:9090/admin/v1/plugins -X PUT -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "UpstreamOutput", "config": {"plugin_name": "test-upstreamoutput1", "target_pipelines": ["test-upstream1", "test-upstream2"], "request_data_keys": ["HTTP_REQUEST_BODY_IO"], "route_policy": "weighted_round_robin", "target_weights": [2, 1]}}'

$ curl -i -k http://127.0.0.1:10080/test -X GET -i -w "\n" -H "name:bar" -d "$LOAD"
HTTP/1.1 200 OK
Date: Mon, 12 Jun 2017 06:26:22 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<html><body>OK - 1122</body></html>
$ curl -i -k http://127.0.0.1:10080/test -X GET -i -w "\n" -H "name:bar" -d "$LOAD"
HTTP/1.1 200 OK
Date: Mon, 12 Jun 2017 06:26:23 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<html><body>OK - 1122</body></html>
$ curl -i -k http://127.0.0.1:10080/test -X GET -i -w "\n" -H "name:bar" -d "$LOAD"
HTTP/1.1 200 OK
Date: Mon, 12 Jun 2017 06:26:24 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<html><body>OK - 3344</body></html>
$ curl -i -k http://127.0.0.1:10080/test -X GET -i -w "\n" -H "name:bar" -d "$LOAD"
HTTP/1.1 200 OK
Date: Mon, 12 Jun 2017 06:26:25 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<html><body>OK - 1122</body></html>
$ curl -i -k http://127.0.0.1:10080/test -X GET -i -w "\n" -H "name:bar" -d "$LOAD"
HTTP/1.1 200 OK
Date: Mon, 12 Jun 2017 06:26:25 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<html><body>OK - 1122</body></html>
$ curl -i -k http://127.0.0.1:10080/test -X GET -i -w "\n" -H "name:bar" -d "$LOAD"
HTTP/1.1 200 OK
Date: Mon, 12 Jun 2017 06:26:26 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<html><body>OK - 3344</body></html>
```

>**Note**:
> * Under `weighted_round_robin` policy, the upstream with a zero weight will not get any chance to handle the request from the downstream. Therefore, if you gives a weight list of only zero value, the gateway will reject the plugin creation or update request with a proper error as a fast-failure. Fox example:
> ```
> $ curl http://127.0.0.1:9090/admin/v1/plugins -X PUT -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "UpstreamOutput", "config": {"plugin_name": "test-upstreamoutput1", "target_pipelines": ["test-upstream1", "test-upstream2"], "request_data_keys": ["HTTP_REQUEST_BODY_IO"], "route_policy": "weighted_round_robin", "target_weights": [0, 0]}}'
> HTTP/1.1 400 Bad Request
> Content-Type: application/json; charset=utf-8
> X-Powered-By: go-json-rest
> Date: Mon, 12 Jun 2017 06:25:29 GMT
> Content-Length: 91
>
> {"Error":"invalid target pipeline weights, one of them should be greater or equal to zero"}
> ```
> * The default weight of each upstream under `weighted_round_robin` policy is value 1, which means the behavior equals `round_robin` policy.

Next, let's see Blue/Green deployment case, this time we need to use `filter` policy and provide a proper condition option to it. The data belongs to the key `QUERY_STRING` is given by HTTP input plugin, you can check the detail out by plugin reference document.

```
$ curl http://127.0.0.1:9090/admin/v1/plugins -X PUT -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "UpstreamOutput", "config": {"plugin_name": "test-upstreamoutput1", "target_pipelines": ["test-upstream1", "test-upstream2"], "request_data_keys": ["HTTP_REQUEST_BODY_IO"], "route_policy": "filter", "filter_conditions": [{"QUERY_STRING": "release=green"}, {"QUERY_STRING": "release=blue"}]}}'

$ curl -i -k http://127.0.0.1:10080/test?release=green -X GET -i -w "\n" -H "name:bar" -d "$LOAD"
HTTP/1.1 200 OK
Date: Tue, 13 Jun 2017 08:02:54 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<html><body>OK - 1122</body></html>
$ curl -i -k http://127.0.0.1:10080/test?release=blue -X GET -i -w "\n" -H "name:bar" -d "$LOAD"
HTTP/1.1 200 OK
Date: Tue, 13 Jun 2017 08:03:02 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<html><body>OK - 3344</body></html>
$ curl -i -k http://127.0.0.1:10080/test?release=green -X GET -i -w "\n" -H "name:bar" -d "$LOAD"
HTTP/1.1 200 OK
Date: Tue, 13 Jun 2017 08:02:55 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<html><body>OK - 1122</body></html>
$ curl -i -k http://127.0.0.1:10080/test?release=blue -X GET -i -w "\n" -H "name:bar" -d "$LOAD"
HTTP/1.1 200 OK
Date: Tue, 13 Jun 2017 08:03:04 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<html><body>OK - 3344</body></html>
$ curl -i -k http://127.0.0.1:10080/test?release=shit -X GET -i -w "\n" -H "name:bar" -d "$LOAD"
HTTP/1.1 503 Service Unavailable
Date: Tue, 13 Jun 2017 08:03:24 GMT
Content-Type: text/plain; charset=utf-8
Transfer-Encoding: chunked

```

Follow is an upstream timeout case.

A fake HTTP Server to serve the request for demo, this time we add a 5 seconds sleep to simulate upstream response timeout.

```
$ cat ~/server3.py

from BaseHTTPServer import BaseHTTPRequestHandler,HTTPServer
import sys
import time

class WebServerHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        time.sleep(5)
        if self.path.endswith("/abc"):
            content_len = int(self.headers.get('Content-Length', 0))
            post_body = self.rfile.read(content_len)
            print content_len, post_body
            self.send_response(200)
            self.send_header('Content-type', 'text/html')
            self.end_headers()
            message = "<html><body>OK - %s</body></html>" % sys.argv[1]
            self.wfile.write(message)
            return

def main():
    try:
        port = int(sys.argv[1])
        server = HTTPServer(('127.0.0.1', port), WebServerHandler)
        print "Web Server running on port %s" % port
        server.serve_forever()
    except KeyboardInterrupt:
        print " ^C entered, stopping web server...."
        server.socket.close()

main()

$ python ~/server3.py 1122
$ python ~/server3.py 3344 # in a different terminal
```

Sending out client requests to the proxy endpoint we created by above commands.

```
$ curl http://127.0.0.1:9090/admin/v1/plugins -X PUT -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "UpstreamOutput", "config": {"plugin_name": "test-upstreamoutput1", "target_pipelines": ["test-upstream1", "test-upstream2"], "request_data_keys": ["HTTP_REQUEST_BODY_IO"], "route_policy": "round_robin", "timeout_sec": 2}}'

$ curl -i -k http://127.0.0.1:10080/test -X GET -i -w "\n" -H "name:bar" -d "$LOAD"
HTTP/1.1 503 Service Unavailable
Date: Mon, 12 Jun 2017 06:40:41 GMT
Content-Type: text/plain; charset=utf-8
Transfer-Encoding: chunked
```

At the moment we can see such logs have been outputted by gateway server like following.

```
WARN[2017-06-12T14:42:02+08:00] [plugin test-upstreamoutput1 in pipeline input execution failure, resultcode=503, error="upstream is timeout after 2 second(s)"]  source="linear.go#186-model.(*linearPipeline).Run"
WARN[2017-06-12T14:42:02+08:00] [http request processed unsuccesfully, result code: 503, error: upstream is timeout after 2 second(s)]  source="http_input.go#382-plugins.(*httpInput).receive.func3"
ERRO[2017-06-12T14:42:05+08:00] [respond downstream pipeline test-upstreamoutput1 failed: request from pipeline test-upstreamoutput1 was closed]  source="downstream_input.go#92-plugins.(*downstreamInput).Run.func1"
```

Finally, let's check upstream retry case. In this case, the request chould be handled by both upstreams, and when #1 upstream returns failure we'd like #2 upstream takes over.

A fake HTTP Server to serve the request for demo, this time we fail the request on the server which listens the port 1122.

```
$ cat ~/server3.py

from BaseHTTPServer import BaseHTTPRequestHandler,HTTPServer
import sys
import time

port = 0

class WebServerHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        global port
        if self.path.endswith("/abc"):
            content_len = int(self.headers.get('Content-Length', 0))
            post_body = self.rfile.read(content_len)
            print content_len, post_body
            if port == 1122:
                self.send_response(500)
            else:
                self.send_response(200)
            self.send_header('Content-type', 'text/html')
            self.end_headers()
            message = "<html><body>OK - %s</body></html>" % sys.argv[1]
            self.wfile.write(message)
            return

def main():
    try:
        global port
        port = int(sys.argv[1])
        server = HTTPServer(('127.0.0.1', port), WebServerHandler)
        print "Web Server running on port %s" % port
        server.serve_forever()
    except KeyboardInterrupt:
        print " ^C entered, stopping web server...."
        server.socket.close()

main()

$ python ~/server3.py 1122
$ python ~/server3.py 3344 # in a different terminal
```

Sending out client requests to the proxy endpoint we created by above commands. The IO reader plugin new added is necessary, because original request IO body can not be read twice if the first try is failed, so need to read request body first and reuse it in second HTTP output.

```
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "IOReader", "config": {"plugin_name": "test-ioreader1", "input_key": "HTTP_REQUEST_BODY_IO", "output_key": "REQ_DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X PUT -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "UpstreamOutput", "config": {"plugin_name": "test-upstreamoutput1", "target_pipelines": ["test-upstream1", "test-upstream2"], "request_data_keys": ["REQ_DATA"], "route_policy": "retry"}}'
$ curl http://127.0.0.1:9090/admin/v1/pipelines -X PUT -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "LinearPipeline", "config": {"pipeline_name": "input", "plugin_names": ["httpserver", "test-httpinput", "test-ioreader1", "test-upstreamoutput1"], "parallelism": 10}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X PUT -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPOutput", "config": {"plugin_name": "test-httpoutput1", "url_pattern": "http://127.0.0.1:1122/abc", "header_patterns": {}, "method": "POST", "response_code_key": "HTTP_RESP_CODE", "response_body_io_key": "HTTP_RESP_BODY_IO", "request_body_buffer_pattern": "{REQ_DATA}", "close_body_after_pipeline": false}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X PUT -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPOutput", "config": {"plugin_name": "test-httpoutput2", "url_pattern": "http://127.0.0.1:3344/abc", "header_patterns": {}, "method": "POST", "response_code_key": "HTTP_RESP_CODE", "response_body_io_key": "HTTP_RESP_BODY_IO", "request_body_buffer_pattern": "{REQ_DATA}", "close_body_after_pipeline": false}}'

$ curl -i -k http://127.0.0.1:10080/test -X GET -i -w "\n" -H "name:bar" -d "$LOAD"
HTTP/1.1 200 OK
Date: Wed, 08 Nov 2017 08:23:09 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<html><body>OK - 3344</body></html>
```

At the moment we can see such logs have been outputted by gateway server like following.

```
WARN[2017-11-08T16:23:09+08:00] [plugin test-httpoutput1 in pipeline test-upstream1 execution failure, resultcode=503, error="http upstream responded with unexpected status code (500)"]  source="linear.go#188-model.(*linearPipeline).Run"
```

## HTTP proxy with caching

In this case, you can see how a pipeline acts a HTTP/HTTPS proxy and how to add a cache layer between input and upstream. The cache function is used to improve the performance of RESTful service automatically.

### Plugin

1. [HTTP server](./plugin_ref.md#http-server-plugin): To enable HTTP server to listen on port 10080.
2. [HTTP input](./plugin_ref.md#http-input-plugin): To enable HTTP endpoint to receive RESTful requests for upstream service.
3. [Simple common cache](./plugin_ref.md#simple-common-cache-plugin): To cache HTTP body upstream service responded. The body buffer will be cached in 10 seconds in this case. During the TTL (Time-To-Live) any request which hits the cache will renew the expiration time of the body buffer automatically.
4. [Simple common cache](./plugin_ref.md#simple-common-cache-plugin): To cache HTTP status code upstream service responded. The status code will be cached in 10 seconds in this case. Like body buffer, during the TTL any request which hits the cache will renew the expiration time of the status code as well.
5. [IO reader](./plugin_ref.md#io-reader-plugin): To read received data from the client via HTTP transport layer in to local memory as a proxy.
6. [HTTP output](./plugin_ref.md#http-output-plugin): Sending the body and headers to a certain endpoint of upstream RESTFul service.
7. [IO reader](./plugin_ref.md#io-reader-plugin): To read response data from the upstream service via HTTP transport layer in to local memory, the body buffer can be cached and will be responded to client in anyway.

Using follow [Administration API](./admin_api_ref.swagger.yaml) calls to setup above plugins:

```
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPServer", "config": {"plugin_name": "httpserver", "port":10080, "keepalive_sec":30}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPInput", "config": {"plugin_name": "test-httpinput", "server_name": "httpserver", "path": "/test", "methods": ["GET"], "headers_enum": {"name": ["bar", "bar1"]}, "request_body_io_key": "HTTP_REQUEST_BODY_IO", "response_code_key": "HTTP_RESP_CODE", "response_body_buffer_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "SimpleCommonCache", "config": {"plugin_name": "test-simplecommoncache-body", "hit_keys": ["HTTP_NAME"], "cache_key": "DATA", "ttl_sec": 10, "finish_if_hit": false}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "SimpleCommonCache", "config": {"plugin_name": "test-simplecommoncache-code", "hit_keys": ["HTTP_NAME"], "cache_key": "HTTP_RESP_CODE", "ttl_sec": 10, "finish_if_hit": true}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "IOReader", "config": {"plugin_name": "test-ioreader1", "input_key": "HTTP_REQUEST_BODY_IO", "output_key": "REQ_DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPOutput", "config": {"plugin_name": "test-httpoutput", "url_pattern": "http://127.0.0.1:1122/abc{fake}", "header_patterns": {}, "method": "GET", "response_code_key": "HTTP_RESP_CODE", "response_body_io_key": "HTTP_RESP_BODY_IO", "request_body_buffer_pattern": "{REQ_DATA}"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "IOReader", "config": {"plugin_name": "test-ioreader2", "input_key": "HTTP_RESP_BODY_IO", "output_key": "DATA"}}'
```

### Pipeline

We need to set both cache plugins to the certain position in the pipeline, in this case we would like to response client from cache as soon as possible since once cache is hit there is no reason to handle the request for rest steps in the pipeline, so we add the cache stuff close to HTTP input plugin.

You can use follow [Administration API](./admin_api_ref.swagger.yaml) calls to setup the pipeline:

```
$ curl http://127.0.0.1:9090/admin/v1/pipelines -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "LinearPipeline", "config": {"pipeline_name": "test-jsonpipeline", "plugin_names": ["httpserver", "test-httpinput", "test-simplecommoncache-body", "test-simplecommoncache-code", "test-ioreader1", "test-httpoutput", "test-ioreader2"], "parallelism": 10}}'
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

In this case, we would like to show a way to make failure upstream service to returns mock data instead of exposing failure to the client directly. In general downstream on client side is a critical service, it means even which depends on the upstream however the business in the service is not important that can be skipped if some issues happened at there, like loading customer comments for a commodity display page.

>**Note**:
> To simulate upstream failure in the example, an assistant plugin is added to the pipeline, you need not it in the real case.

### Plugins

1. [HTTP server](./plugin_ref.md#http-server-plugin): To enable HTTP server to listen on port 10080.
2. [HTTP input](./plugin_ref.md#http-input-plugin): To enable HTTPS endpoint to receive Ease Monitor data sent from the client.
3. [Simple common mock](./plugin_ref.md#simple-common-mock-plugin): To returns mock data for the failure Ease Monitor service at upstream.
4. [Static pass probability limiter](./plugin_ref.md#static-pass-probability-limiter-plugin): The plugin passes the request with a fixed probability, in this case it is used to simulate upstream service failure with 50% probability.
5. [IO reader](./plugin_ref.md#io-reader-plugin): To read Ease Monitor data from the client via HTTPS transport layer in to local memory for handling in next steps.
6. [JSON validator](./plugin_ref.md#json-validator-plugin): Validating the Ease Monitor data sent from the client is using a certain schema. You can use [Ease Monitor graphite validator](./plugin_ref.md#ease-monitor-graphite-validator-plugin) if you would like to use the pipeline to handle Ease Monitor data with graphite plaintext protocol.
7. [Ease Monitor JSON GID extractor](./plugin_ref.md#ease-monitor-json-gid-extractor-plugin): Extracts Ease Monitor global ID from the Ease Monitor data. You can use [Ease Monitor graphite GID extractor](./plugin_ref.md#ease-monitor-graphite-gid-extractor-plugin) if you would like to use the pipeline to handle Ease Monitor data with graphite plaintext protocol.
8. [Kafka output](./plugin_ref.md#kafka-output-plugin): Sending the data to configured kafka topic, Ease Monitor pipeline will fetch them for rest of processes.

Using follow [Administration API](./admin_api_ref.swagger.yaml) calls to setup above plugins:

```
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPServer", "config": {"plugin_name": "httpserver", "port":10080, "keepalive_sec":30}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPInput", "config": {"plugin_name": "test-httpinput", "server_name": "httpserver", "path": "/test", "methods": ["POST"], "headers_enum": {"name": ["bar", "bar1"]}, "request_body_io_key": "HTTP_REQUEST_BODY_IO"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "SimpleCommonMock", "config": {"plugin_name": "test-simplecommonmock", "plugin_concerned": "test-staticprobabilitylimiter", "task_error_codes_concerned": ["ResultFlowControl"], "mock_task_data_key": "example", "mock_task_data_value": "fake"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "StaticProbabilityLimiter", "config": {"plugin_name": "test-staticprobabilitylimiter", "pass_pr": 0.5}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "IOReader", "config": {"plugin_name": "test-ioreader", "input_key": "HTTP_REQUEST_BODY_IO", "output_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "JSONValidator", "config": {"plugin_name": "test-jsonvalidator", "schema": "{\"title\": \"Record\",\"type\": \"object\",\"properties\": {\"name\": {\"type\": \"string\"}}, \"required\": [\"name\"]}", "data_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "EaseMonitorJSONGidExtractor", "config": {"plugin_name": "test-jsongidextractor", "gid_key": "GID", "data_key": "DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "KafkaOutput", "config": {"plugin_name": "test-kafkaoutput", "topic": "test", "brokers": ["127.0.0.1:9092"], "message_key_key": "GID", "data_key": "DATA"}}'
```

### Pipeline

You can use follow [Administration API](./admin_api_ref.swagger.yaml) calls to setup the pipeline:

```
$ curl http://127.0.0.1:9090/admin/v1/pipelines -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "LinearPipeline", "config": {"pipeline_name": "test-jsonpipeline", "plugin_names": ["httpserver", "test-httpinput", "test-simplecommonmock", "test-staticprobabilitylimiter", "test-ioreader", "test-jsonvalidator", "test-jsongidextractor", "test-kafkaoutput"], "parallelism": 10}}'
```

### Test

In output of ApacheBench you can see there are no any Non-2xx responses.

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

In a flash sale event of e-Commerce case, the behaviors between user and website are like these:

1. Before the flash sale event starts, many users open the event page of the website and prepare to click the order link. At the moment the order link is under disable status since event is not started.
2. After the flash sale event starts, users click the order link and a large number of the order requests are sent to the website, and website handles the requests as much as possible on the basis of the service availability.
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

1. [HTTP server](./plugin_ref.md#http-server-plugin): To enable HTTP server to listen on port 10080.
2. [HTTP input](./plugin_ref.md#http-input-plugin): To enable HTTP endpoint to receive user request for accessing flash sale event page.
3. [IO reader](./plugin_ref.md#io-reader-plugin): To read received data from the client via HTTP transport layer in to local memory as a proxy.
4. [HTTP output](./plugin_ref.md#http-output-plugin): Sending the body and headers to the flash sale event page of the website.
5. [HTTP header counter](./plugin_ref.md#http-header-counter-plugin): To calculate amount of different HTTP header value of a certain header name, and the amount will be deducted automatically if user has not any active with website more than 1 minute. The result is exposed by the statistics indicator ``RECENT_HEADER_COUNT``.

Using follow [Administration API](./admin_api_ref.swagger.yaml) calls to setup above plugins:

```
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPServer", "config": {"plugin_name": "httpserver", "port":10080, "keepalive_sec":30}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPInput", "config": {"plugin_name": "test-httpinput1", "server_name": "httpserver", "path": "/test/book", "methods": ["GET"], "headers_enum": {}, "request_body_io_key": "HTTP_REQUEST_BODY_IO", "response_code_key": "HTTP_RESP_CODE", "response_body_io_key": "HTTP_RESP_BODY_IO"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "IOReader", "config": {"plugin_name": "test-ioreader1", "input_key": "HTTP_REQUEST_BODY_IO", "output_key": "REQ_DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPOutput", "config": {"plugin_name": "test-httpoutput1", "url_pattern": "http://127.0.0.1:1122/book/abc", "header_patterns": {}, "method": "GET", "response_code_key": "HTTP_RESP_CODE", "response_body_io_key": "HTTP_RESP_BODY_IO", "request_body_buffer_pattern": "{REQ_DATA}"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPHeaderCounter", "config": {"plugin_name": "test-httpheadercounter1", "header_concerned": "name", "expiration_sec": 60}}'
```

#### Pipeline

Technically there is no requirement on the position of HTTP header counter plugin.

You can use follow [Administration API](./admin_api_ref.swagger.yaml) calls to setup the pipeline:

```
$ curl http://127.0.0.1:9090/admin/v1/pipelines -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "LinearPipeline", "config": {"pipeline_name": "test-jsonpipeline1", "plugin_names": ["httpserver", test-httpinput1", "test-ioreader1", "test-httpoutput1", "test-httpheadercounter1"], "parallelism": 10}}'
```

#### Test
```
$ cat ~/server.py
from BaseHTTPServer import BaseHTTPRequestHandler,HTTPServer

global stock
stock = 6

class WebServerHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path.endswith("/book/abc"):
                self.send_response(200)
                self.send_header('Content-type', 'text/html')
                self.end_headers()
                message = "<html><body>Book successfully!</body></html>"
                self.wfile.write(message)
        elif self.path.endswith("/abc"):
            global stock
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
            stock -= 1

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
$ python ~/server.py
Web Server running on port 1122
127.0.0.1 - - [24/Jul/2017 15:48:19] "GET /book/abc HTTP/1.1" 200 -
127.0.0.1 - - [24/Jul/2017 15:48:24] "GET /book/abc HTTP/1.1" 200 -
127.0.0.1 - - [24/Jul/2017 15:48:43] "GET /book/abc HTTP/1.1" 200 -
127.0.0.1 - - [24/Jul/2017 15:51:36] "GET /book/abc HTTP/1.1" 200 -
```

```
$ curl http://127.0.0.1:9090/statistics/v1/pipelines/test-jsonpipeline/plugins/test-httpheadercounter/indicators/RECENT_HEADER_COUNT/desc  -X GET -w "\n"
{"desc":"The count of http requests that the header of each one contains a key 'name' in last 60 second(s)."}

$ curl http://127.0.0.1:9090/statistics/v1/pipelines/test-jsonpipeline/plugins/test-httpheadercounter/indicators/RECENT_HEADER_COUNT/value  -X GET -w "\n"
{"value":0}

$ curl -i -k https://127.0.0.1:10443/test/book -X GET -i -w "\n" -H "name:bar1" -d "$LOAD"
HTTP/1.1 200 OK
Date: Mon, 24 Jul 2017 08:15:03 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

$ curl -i -k https://127.0.0.1:10443/test/book -X GET -i -w "\n" -H "name:bar2" -d "$LOAD"
HTTP/1.1 200 OK
Date: Mon, 24 Jul 2017 08:15:06 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

$ curl -i -k https://127.0.0.1:10443/test/book -X GET -i -w "\n" -H "name:bar3" -d "$LOAD"
HTTP/1.1 200 OK
Date: Mon, 24 Jul 2017 08:15:09 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

$ curl -i -k https://127.0.0.1:10443/test/book -X GET -i -w "\n" -H "name:bar4" -d "$LOAD"
HTTP/1.1 200 OK
Date: Mon, 24 Jul 2017 08:15:13 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

$ curl http://127.0.0.1:9090/statistics/v1/pipelines/test-jsonpipeline1/plugins/test-httpheadercounter1/indicators/RECENT_HEADER_COUNT/value  -X GET -w "\n"
{"value":4}
```

### Rejecting requests

#### Plugin

1. [HTTP server](./plugin_ref.md#http-server-plugin): To enable HTTP server to listen on port 10080.
2. [HTTP input](./plugin_ref.md#http-input-plugin): To enable HTTP endpoint to receive user request for order commodity.
3. [No more failure limiter](./plugin_ref.md#no-more-failure-limiter-plugin): To return a standard error and reject all incoming requests directly and no further requests hit the upstream.
4. [Static pass probability limiter](./plugin_ref.md#static-pass-probability-limiter-plugin): The plugin passes the request with a fixed probability. In this case it is used to reduce the massive order requests and finally a reasonable volume of order requests hit the website backend service really.
5. [IO reader](./plugin_ref.md#io-reader-plugin): To read received data from the client via HTTP transport layer in to local memory as a proxy.
6. [HTTP output](./plugin_ref.md#http-output-plugin): Sending the body and headers to the order service of the website.

```
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPServer", "config": {"plugin_name": "httpserver", "port":10080, "keepalive_sec":30}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPInput", "config": {"plugin_name": "test-httpinput2", "server_name": "httpserver", "path": "/test", "methods": ["GET"], "headers_enum": {}, "request_body_io_key": "HTTP_REQUEST_BODY_IO", "response_code_key": "HTTP_RESP_CODE", "response_body_io_key": "HTTP_RESP_BODY_IO"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "NoMoreFailureLimiter", "config": {"plugin_name": "test-nomorefailurelimiter2", "failure_count_threshold": 1, "failure_task_data_key": "HTTP_RESP_CODE", "failure_task_data_value": "400"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "StaticProbabilityLimiter", "config": {"plugin_name": "test-staticprobabilitylimiter2", "pass_pr": 0.75}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "IOReader", "config": {"plugin_name": "test-ioreader2", "input_key": "HTTP_REQUEST_BODY_IO", "output_key": "REQ_DATA"}}'
$ curl http://127.0.0.1:9090/admin/v1/plugins -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "HTTPOutput", "config": {"plugin_name": "test-httpoutput2", "url_pattern": "http://127.0.0.1:1122/abc", "header_patterns": {}, "method": "GET", "response_code_key": "HTTP_RESP_CODE", "response_body_io_key": "HTTP_RESP_BODY_IO", "request_body_buffer_pattern": "{REQ_DATA}"}}'
```

#### Pipeline

You can use follow [Administration API](./admin_api_ref.swagger.yaml) calls to setup the pipeline:

```
$ curl http://127.0.0.1:9090/admin/v1/pipelines -X POST -i -H "Content-Type:application/json" -H "Accept:application/json" -w "\n" -d '{"type": "LinearPipeline", "config": {"pipeline_name": "test-jsonpipeline2", "plugin_names": ["httpserver", "test-httpinput2", "test-nomorefailurelimiter2", "test-staticprobabilitylimiter2", "test-ioreader2", "test-httpoutput2"], "parallelism": 10}}'
```

#### Test

```
python server.py
Web Server running on port 1122
127.0.0.1 - - [24/Jul/2017 16:20:13] "GET /abc HTTP/1.1" 200 -
127.0.0.1 - - [24/Jul/2017 16:20:16] "GET /abc HTTP/1.1" 200 -
127.0.0.1 - - [24/Jul/2017 16:20:19] "GET /abc HTTP/1.1" 200 -
127.0.0.1 - - [24/Jul/2017 16:20:22] "GET /abc HTTP/1.1" 200 -
127.0.0.1 - - [24/Jul/2017 16:20:25] "GET /abc HTTP/1.1" 200 -
127.0.0.1 - - [24/Jul/2017 16:20:27] "GET /abc HTTP/1.1" 200 -
127.0.0.1 - - [24/Jul/2017 16:20:30] "GET /abc HTTP/1.1" 400 -
```

```
$ curl -i -k https://127.0.0.1:10443/test -X GET -i -w "\n" -H "name:bar1" -d "$LOAD"
HTTP/1.1 200 OK
Date: Mon, 24 Jul 2017 08:20:13 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<html><body>Order successfully!</body></html>

$ curl -i -k https://127.0.0.1:10443/test -X GET -i -w "\n" -H "name:bar2" -d "$LOAD"
HTTP/1.1 200 OK
Date: Mon, 24 Jul 2017 08:20:16 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<html><body>Order successfully!</body></html>

$ curl -i -k https://127.0.0.1:10443/test -X GET -i -w "\n" -H "name:bar3" -d "$LOAD"
HTTP/1.1 200 OK
Date: Mon, 24 Jul 2017 08:20:19 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<html><body>Order successfully!</body></html>

$ curl -i -k https://127.0.0.1:10443/test -X GET -i -w "\n" -H "name:bar4" -d "$LOAD"
HTTP/1.1 200 OK
Date: Mon, 24 Jul 2017 08:20:22 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<html><body>Order successfully!</body></html>

$ curl -i -k https://127.0.0.1:10443/test -X GET -i -w "\n" -H "name:bar5" -d "$LOAD"
HTTP/1.1 200 OK
Date: Mon, 24 Jul 2017 08:20:25 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<html><body>Order successfully!</body></html>

$ curl -i -k https://127.0.0.1:10443/test -X GET -i -w "\n" -H "name:bar6" -d "$LOAD"
HTTP/1.1 200 OK
Date: Mon, 24 Jul 2017 08:20:27 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<html><body>Order successfully!</body></html>

$ curl -i -k https://127.0.0.1:10443/test -X GET -i -w "\n" -H "name:bar7" -d "$LOAD"
HTTP/1.1 400 Bad Request
Date: Mon, 24 Jul 2017 08:20:30 GMT
Content-Type: text/html; charset=utf-8
Transfer-Encoding: chunked

<html><body>Order failed!</body></html>

$ curl -i -k https://127.0.0.1:10443/test -X GET -i -w "\n" -H "name:bar8" -d "$LOAD"
HTTP/1.1 429 Too Many Requests
Date: Mon, 24 Jul 2017 08:20:33 GMT
Content-Type: text/plain; charset=utf-8
Transfer-Encoding: chunked
```
