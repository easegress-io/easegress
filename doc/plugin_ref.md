Ease Gateway Plugin Reference
===============================

## Summary

There are 16 available plugins totoally in Ease Gateway current release.

| Plugin name | Type name | Block-able | Functional | Development status | Link |
|:--|:--|:--:|:--:|:--:|:--:|:--|
| Http input | HttpInput | Yes | Yes | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/plugins/http_input.go) |
| Graphite validator | GraphiteValidator | No | Yes | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/plugins/graphite_validator.go) |
| Json validator | JSONValidator | No | Yes | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/plugins/json_validator.go) |
| Graphite GID extractor | GraphiteGidExtractor | No | Yes | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/plugins/graphite_gid_extractor.go) |
| Json GID extractor | JSONGidExtractor | No | Yes | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/plugins/json_gid_extractor.go) |
| Kafka output | KafkaOutput | No | Yes | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/plugins/kafka_output.go) |
| Http output | HTTPOutput | No | Yes  | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/plugins/http_output.go) |
| IO reader | IOReader | Yes | Yes | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/plugins/io_reader.go) |
| Http header counter | HTTPHeaderCounter | No | No | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/plugins/http_header_counter.go) |
| Throughput rate limiter | ThroughputRateLimiter | No | No | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/plugins/throughput_rate_limiter.go) |
| Latency based sliding window limiter | LatencyWindowLimiter | No | No | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/plugins/latency_window_limiter.go) |
| Service circuit breaker | ServiceCircuitBreaker | No | No | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/plugins/service_circuit_breaker.go) |
| Static pass probability limiter | StaticProbabilityLimiter | No | No | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/plugins/static_probability_limiter.go) |
| No more failure limiter | NoMoreFailureLimiter | No | No | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/plugins/no_more_failure_limiter.go) |
| Simple common cache | SimpleCommonCache | No | No | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/plugins/simple_common_cache.go) |
| Simple common mock | SimpleCommonMock | No | No | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/plugins/simple_common_mock.go) |

### Http Input Plugin

Plugin handles HTTP request and retruns client with pipeline procssed response.
 
#### Configuration

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--|
| url | string | The request HTTP url plugin will proceed. | Functionality | No | N/A |
| method | string | The request HTTP method plugin will proceed. | Functionality | Yes | "GET" |
| headers_enum | map[string]string | The request HTTP headers plugin will proceed. | Functionality | No | N/A |
| unzip | bool | The flag represents if the plugin decompresses the request body when request content is encoded in GZIP. | Functionality | Yes | true |
| respond_error | bool | The flag represents if the plugin respond error information to client if pipeline handles the request unsuccessfully. The option will be used only when `response_body_io_key` and `response_body_io_key` options are empty. | Functionality | Yes | false |
| request\_body\_io\_key | string | The key name of request body io object stored in internal storage for the plugin output. | I/O | Yes | "" |
| response\_code\_key | string | The key name of response code value stored in internal storage for the plugin output. An empty value of the option means returning pipeline handling result code to client. | I/O | Yes | "" |
| response\_body\_io\_key | string | The key name of response body io object stored in internal storage for the plugin output. | I/O | Yes | "" |
| response\_body\_buffer\_key | string | The key name of response body buffer stored in internal storage for the plugin output. The option will be leveraged only when `response_body_io_key` option is empty. | I/O | Yes | "" |

#### I/O

| Data name | Configuration option name | Type | Optional |
|:--|:--|:--:|:--:|:--|
| Request body IO object | request\_body\_io\_key | Output | Yes |
| Response http status code | response\_code\_key | Input | Yes |
| Response body IO object | response\_body\_io\_key | Input | Yes |
| Response body buffer | response\_body\_buffer\_key | Input | Yes |

#### Error

| Result code | Error reason |
|:--|:--|
| ResultRequesterGone | client closed |

#### Dedicated statistics indicator

| Indicator name | Data type (golang) | Description |
|:--:|:--:|:--|
| WAIT\_QUEUE\_LENGTH | uint64 | The length of wait queue which contains requests wait to be handled by a pipeline. |
| WIP\_REQUEST\_COUNT | uint64 | The count of request which in the working progress of the pipeline. |