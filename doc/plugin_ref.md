Ease Gateway Plugin Reference
===============================

## Summary

There are 16 available plugins totoally in Ease Gateway current release.

| Plugin name | Type name | Block-able | Functional | Development status | Link |
|:--|:--|:--:|:--:|:--:|:--|
| [Http input](#http-input-plugin) | HttpInput | Yes | Yes | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/plugins/http_input.go) |
| [Graphite validator](#graphite-validator-plugin) | GraphiteValidator | No | Yes | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/plugins/graphite_validator.go) |
| [Json validator](#json-validator-plugin) | JSONValidator | No | Yes | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/plugins/json_validator.go) |
| [Graphite GID extractor](#graphite-gid-extractor-plugin) | GraphiteGidExtractor | No | Yes | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/plugins/graphite_gid_extractor.go) |
| [Json GID extractor](#json-gid-extractor-plugin) | JSONGidExtractor | No | Yes | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/plugins/json_gid_extractor.go) |
| [Kafka output](#kafka-output-plugin) | KafkaOutput | No | Yes | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/plugins/kafka_output.go) |
| [Http output](#http-output-plugin) | HTTPOutput | No | Yes  | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/plugins/http_output.go) |
| [IO reader](#io-reader-plugin) | IOReader | Yes | Yes | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/plugins/io_reader.go) |
| [Http header counter](#http-header-counter-plugin) | HTTPHeaderCounter | No | No | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/plugins/http_header_counter.go) |
| [Throughput rate limiter](#throughput-rate-limiter-plugin) | ThroughputRateLimiter | Yes | No | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/plugins/throughput_rate_limiter.go) |
| [Latency based sliding window limiter](#latency-based-sliding-window-limiter-plugin) | LatencyWindowLimiter | Yes | No | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/plugins/latency_window_limiter.go) |
| [Service circuit breaker](#service-circuit-breaker-plugin) | ServiceCircuitBreaker | No | No | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/plugins/service_circuit_breaker.go) |
| [Static pass probability limiter](#static-pass-probability-limiter-plugin) | StaticProbabilityLimiter | No | No | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/plugins/static_probability_limiter.go) |
| [No more failure limiter](#no-more-failure-limiter-plugin) | NoMoreFailureLimiter | No | No | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/plugins/no_more_failure_limiter.go) |
| [Simple common cache](#simple-common-cache-plugin) | SimpleCommonCache | No | No | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/plugins/simple_common_cache.go) |
| [Simple common mock](#simple-common-mock-plugin) | SimpleCommonMock | No | No | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/plugins/simple_common_mock.go) |

## Http Input plugin

Plugin handles HTTP request and retruns client with pipeline procssed response. Currently a HTTPS server will runs on a fixed 10443 port with a certificate and key file pair.
 
### Configuration

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--|
| plugin\_name | string | The plugin instance name. | Functionality | No | N/A |
| url | string | The request HTTP url plugin will proceed. | Functionality | No | N/A |
| method | string | The request HTTP method plugin will proceed. | Functionality | Yes | "GET" |
| headers_enum | map[string]string | The request HTTP headers plugin will proceed. | Functionality | No | N/A |
| unzip | bool | The flag represents if the plugin decompresses the request body when request content is encoded in GZIP. | Functionality | Yes | true |
| respond_error | bool | The flag represents if the plugin respond error information to client if pipeline handles the request unsuccessfully. The option will be used only when `response_body_io_key` and `response_body_io_key` options are empty. | Functionality | Yes | false |
| request\_body\_io\_key | string | The key name of http request body io object stored in internal storage as the plugin output. | I/O | Yes | "" |
| response\_code\_key | string | The key name of http response status code value stored in internal storage as the plugin input. An empty value of the option means returning pipeline handling result code to client. | I/O | Yes | "" |
| response\_body\_io\_key | string | The key name of http response body io object stored in internal storage as the plugin input. | I/O | Yes | "" |
| response\_body\_buffer\_key | string | The key name of http response body buffer stored in internal storage as the plugin input. The option will be leveraged only when `response_body_io_key` option is empty. | I/O | Yes | "" |

### I/O

| Data name | Configuration option name | Type | Data Type | Optional |
|:--|:--|:--:|:--|:--:|
| Request body IO object | request\_body\_io\_key | Output | io.Reader | Yes |
| Response http status code | response\_code\_key | Input | int | Yes |
| Response body IO object | response\_body\_io\_key | Input | io.Reader | Yes |
| Response body buffer | response\_body\_buffer\_key | Input | []byte | Yes |

### Error

| Result code | Error reason |
|:--|:--|
| ResultRequesterGone | client closed |
| ResultTaskCancelled | task is cancelled |

### Dedicated statistics indicator

| Indicator name | Data type (golang) | Description |
|:--:|:--:|:--|
| WAIT\_QUEUE\_LENGTH | uint64 | The length of wait queue which contains requests wait to be handled by a pipeline. |
| WIP\_REQUEST\_COUNT | uint64 | The count of request which in the working progress of the pipeline. |

## Graphite Validator plugin

Plugin validates input data, to check if it's a valid graphite data with plaintext protocol.

### Configuration

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--|
| plugin\_name | string | The plugin instance name. | Functionality | No | N/A |
| data\_key | string | The key name of data needs to check as the plugin input. | I/O | No | N/A |

### I/O

| Data name | Configuration option name | Type | Data Type  | Optional |
|:--|:--|:--:|:--|:--:|
| Data | data\_key | Input | string | No |

### Error

| Result code | Error reason |
|:--|:--|
| ResultBadInput | graphite data got EOF |
| ResultBadInput | graphite data want 4 fields('#'-splitted) |

### Dedicated statistics indicator

No any indicators exposed.

## Json Validator plugin

Plugin validates input data, to check if it's a valid json data with a special schema.

### Configuration

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--|
| plugin\_name | string | The plugin instance name. | Functionality | No | N/A |
| schema | string | The schema json data needs to accord. | Functionality | No | N/A |
| data\_key | string | The key name of data needs to check as the plugin input. | I/O | No | N/A |

### I/O

| Data name | Configuration option name | Type | Data Type | Optional |
|:--|:--|:--:|:--|:--:|
| Data | data\_key | Input | string | No |

### Error

| Result code | Error reason |
|:--|:--|
| ResultInternalServerError | schema not found |
| ResultMissingInput | input got wrong value |
| ResultBadInput | failed to validate |

### Dedicated statistics indicator

No any indicators exposed.

## Graphite GID Extractor plugin

Plugin extracts Ease Monitor global ID from graphite data.

### Configuration

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--|
| plugin\_name | string | The plugin instance name. | Functionality | No | N/A |
| gid\_key | string | The key name of global ID stored in internal storage as the plugin output. | I/O | No | N/A |
| data\_key | string | The key name of data needs to be extracted as the plugin input. | I/O | No | N/A |

### I/O

| Data name | Configuration option name | Type | Data Type | Optional |
|:--|:--|:--:|:--|:--:|
| Global ID | gid\_key | Output | string | No |
| Data | data\_key | Input | string | No |

### Error

| Result code | Error reason |
|:--|:--|
| ResultMissingInput | input got wrong value |
| ResultBadInput | unexpected EOF |
| ResultBadInput | graphite data want 4 fields('#'-splitted) |
| ResultInternalServerError | failed to output global ID |

### Dedicated statistics indicator

No any indicators exposed.

## Json GID extractor plugin

Plugin extracts Ease Monitor global ID from json data.

### Configuration

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--|
| plugin\_name | string | The plugin instance name. | Functionality | No | N/A |
| gid\_key | string | The key name of global ID stored in internal storage as the plugin output. | I/O | No | N/A |
| data\_key | string | The key name of data needs to be extracted as the plugin input. | I/O | No | N/A |

### I/O

| Data name | Configuration option name | Type | Data Type | Optional |
|:--|:--|:--:|:--|:--:|
| Global ID | gid\_key | Output | string | No |
| Data | data\_key | Input | string | No |

### Error

| Result code | Error reason |
|:--|:--|
| ResultMissingInput | input got wrong value |
| ResultBadInput | invalid json |
| ResultInternalServerError | failed to output global ID |

### Dedicated statistics indicator

No any indicators exposed.

## Kafka Output plugin

Plugin outputs request data to a kafka service.
 
### Configuration

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--|
| plugin\_name | string | The plugin instance name. | Functionality | No | N/A |
| brokers | []string | The kafka broker list. | Functionality | No | N/A |
| client\_id | string | The client id as a kafka consumer. | Functionality | Yes | "easegateway" |
| topic | string | The topic data outputs to. | Functionality | No | N/A |
| message\_key\_key | The key name of message key value stored in internal storage as the plugin input. | I/O | Yes | nil |
| data\_key | The key name of message data stored in internal storage as the plugin input. | I/O | No | N/A |

### I/O

| Data name | Configuration option name | Type | Data Type | Optional |
|:--|:--|:--:|:--|:--:|
| Message key | message\_key\_key | Input | []byte | Yes |
| Message data | data\_key | Input | []byte | No |

### Error

| Result code | Error reason |
|:--|:--|
| ResultServiceUnavailable | kafka producer not ready |
| ResultMissingInput | input got wrong value |
| ResultBadInput | input got empty string |
| ResultServiceUnavailable | failed to send message to the kafka topic |

### Dedicated statistics indicator

No any indicators exposed.

## Http Output plugin

Plugin outputs request data to a HTTP endpoint.

### Configuration

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--|
| plugin\_name | string | The plugin instance name. | Functionality | No | N/A |
| url\_pattern | string | The pattern of the complete HTTP output endpoint. E.g. https://1.2.3.4/abc?def={INPUT_DATA} | Functionality | No | N/A |
| header\_patterns | map[string]string | The list of HTTP output header name pattern and value pattern pair. | Functionality | Yes | {} |
| body\_pattern | string | The HTTP output body pattern. | Functionality | Yes | "" |
| method | string | The method HTTP output used. | Functionality | No | N/A |
| timeout\_sec | uint16 | The request timtout HTTP output limited in second. | Functionality | Yes | 120 |
| cert\_file | string | The certificate file HTTPS output used. | Functionality | Yes | "" |
| key\_file | string | The key file HTTPS output used. | Functionality | Yes | "" |
| ca\_file | string | The root certificate HTTPS output used. | Functionality | Yes | "" |
| insecure\_tls | bool | The flag represents if the plugin does not check server certificate. | Functionality | Yes | false |
| response\_code\_key | string | The key name of http response status code value stored in internal storage as the plugin output. An empty value of the option means the plugin does not output http response status code. | I/O | Yes | "" |
| response\_body\_io\_key | string | The key name of http response body io object stored in internal storage as the plugin output. An empty value of the option means the plugin does not output http response body io object.| I/O | Yes | "" |

### I/O

| Data name | Configuration option name | Type | Data Type | Optional |
|:--|:--|:--:|:--|:--:|
| Response http status code | response\_code\_key | Output | int | Yes |
| Response body IO object | response\_body\_io\_key | Output | io.Reader | Yes |

### Error

| Result code | Error reason |
|:--|:--|
| ResultServiceUnavailable | failed to send http request |
| ResultInternalServerError | failed to output response http status code |
| ResultInternalServerError | failed to output response body IO object |

### Dedicated statistics indicator

No any indicators exposed.

## IO Reader plugin

Plugin reads a given I/O object and output the data.

### Configuration

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--|
| plugin\_name | string | The plugin instance name. | Functionality | No | N/A |
| read\_length\_max | int64 | Maximal bytes to read. | Functionality | Yes | 1048576 (1 MiB) |
| close\_after\_read | bool | The flag represents if to close IO object after reading. | Functionality | Yes | true |
| data\_key | string | The key name of read out data as the plugin output. | I/O | No | N/A |
| input\_key | string | The key name of IO object stored in internal storage as the plugin input. | I/O | No | N/A |

### I/O

| Data name | Configuration option name | Type | Data Type | Optional |
|:--|:--|:--:|:--|:--:|
| Data buffer read out | data\_key | Output | []byte | No |
| IO object to read | gid\_key | Input | io.ReadCloser | No |

### Error

| Result code | Error reason |
|:--|:--|
| ResultMissingInput | input got wrong value |
| ResultInternalServerError | failed to read data |
| ResultInternalServerError | failed to output data buffer |

### Dedicated statistics indicator

No any indicators exposed.

## Http Header Counter plugin

Plugin calculates request count in a recent preidor which has a special header name. This behavior likes count session amount.

### Configuration

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--|
| plugin\_name | string | The plugin instance name. | Functionality | No | N/A |
| header\_concerned | string | The header name plugin calculates. | Functionality | No | N/A |
| expiration\_sec | uint32 | The recent preidor in second. | Functionality | No | N/A |

### I/O

No any inputs or outputs.

### Error

| Result code | Error reason |
|:--|:--|
| ResultMissingInput | input got wrong value |

### Dedicated statistics indicator

| Indicator name | Data type (golang) | Description |
|:--:|:--:|:--|
| RECENT\_HEADER\_COUNT | uint64 | The count of http requests that the header of each one contains the key in the recent preidor. |

## Throughput Rate Limiter plugin

Plugin limits request rate based on current throughput.

### Configuration

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--|
| plugin\_name | string | The plugin instance name. | Functionality | No | N/A |
| tps | The maximal requests pre second. Value -1 means no limition. Value zero here means there is no request could be processed.| Functionality | No | N/A |

### I/O

No any inputs or outputs.

### Error

| Result code | Error reason |
|:--|:--|
| ResultFlowControl | service is unavaialbe caused by throughput rate limit |
| ResultTaskCancelled | task is cancelled |
| ResultInternalServerError | unexpected error on internal delay timer |

### Dedicated statistics indicator

No any indicators exposed.

## Latency Based Sliding Window Limiter plugin

Plugin limits request rate based on current latency based sliding winidow.

### Configuration

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--|
| plugin\_name | string | The plugin instance name. | Functionality | No | N/A |
| plugins\_concerned | []string | Plugins their processing latency will be considered to calculate sliding window size. | Functionality | No | N/A |
| latency\_threshold\_msec | uint32 | The latency threshold in millisecond, when the latency greater than it the sliding window will be shrank. | Functionality | Yes | 800 |
| backoff\_msec | uint16 | How many milliseconds the request need to be delayed when current sliding window is fully closed. | Functionality | Yes | 100 |
| window\_size\_max | uint64 | Maximal sliding window size. | Functionality | Yes | 65535 |
| windows\_size\_init | uint64 | Initial sliding window size. | Functionality | Yes | 512 |

### I/O

No any inputs or outputs.

### Error

| Result code | Error reason |
|:--|:--|
| ResultFlowControl | service is unavaialbe caused by sliding window limit |
| ResultTaskCancelled | task is cancelled |

### Dedicated statistics indicator

No any indicators exposed.

## Service Circuit Breaker plugin

Plugin limits request rate based on current latency based failures.

### Configuration

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--|
| plugin\_name | string | The plugin instance name. | Functionality | No | N/A |
| plugins\_concerned | []string | Plugins their processing failure will be considered to control circuit breaker status. | Functionality | No | N/A |
| all\_tps\_threshold\_to\_enable | float64 | As the condition, it indicates how many requests pre second will cause circuit breaker to be enabled. Value zero means to enable circuit breaker immediately when a request arrived. | Functionality | Yes | 1 |
| failure\_tps\_threshold\_to\_break | float64 | As the condition, it indicates how many failure requests pre second will cause circuit breaker to be turned on. It means fully close request flow. Value zero here means breaker will keep open or half-open status. | Functionality | Yes | 1 |
| failure\_tps\_percent\_threshold\_to\_break | float32 | As the condition, it indicates what percent of failure requests pre second will cause circuit breaker to be turned on. It means fully close request flow. Value zero here means breaker will keep open or half-open status. The option can be leveraged only when `failure\_tps\_threshold\_to\_break` contiditon does not satisfy. | Functionality | No | N/A |
| recovery\_time\_msec | uint32 | As the condition, it indicates how long delay in milliseconds will cause circuit breaker to be turned to half-open status, the status is used to try service availability. In general, it equals to MTTR. | Functionality | Yes | 1000 |
| success\_tps\_threshold\_to\_open | float64 | As the condition, it indicates how many success requests pre second will cause circuit breaker to be turned off. It means fully open request flow. Value zero here means to fully open request flow immediately after recovery time elapsed. | Functionality | Yes | 1 |

### I/O

No any inputs or outputs.

### Error

| Result code | Error reason |
|:--|:--|
| ResultFlowControl | service is unavaialbe caused by service fusing |

### Dedicated statistics indicator

No any indicators exposed.

## Static Pass Probability Limiter plugin

Plugin limits request rate based on static passing probability.

### Configuration

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--|
| plugin\_name | string | The plugin instance name. | Functionality | No | N/A |
| pass\_pr | float32 | The passing probability. Value zero means no request could be processed, and value 1.0 means no request could be limited. | Functionality | No | N/A |

### I/O

No any inputs or outputs.

### Error

| Result code | Error reason |
|:--|:--|
| ResultFlowControl | service is unavaialbe caused by probability limit |

### Dedicated statistics indicator

No any indicators exposed.

## No More Failure Limiter plugin

Plugin limits how many fail requests can be returned before block all follow requests.

### Configuration

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--|
| plugin\_name | string | The plugin instance name. | Functionality | No | N/A |
| failure\_task\_data\_key | string | The key name of the data we check if it is a concerned failure task. | Functionality | No | N/A |
| failure\_task\_data\_value | string | The value of the data we check if it is a concerned failure task. | Functionality | No | N/A |

### I/O

No any inputs or outputs.

### Error

| Result code | Error reason |
|:--|:--|
| ResultFlowControl | service is unavaialbe caused by failure limitation |

### Dedicated statistics indicator

No any indicators exposed.

## Simple Common Cache plugin

Plugin caches a data and uses it to serve follow requests directly.

### Configuration

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--|
| plugin\_name | string | The plugin instance name. | Functionality | No | N/A |
| hit\_keys | []string | All the data with every keys will be considered to check if request hits the cache or missing | Functionality  | No | N/A|
| cache\_key | string | The data with the key will be cached in internal storage as the plugin input (caching) and output (reusing) | I/O | No | N/A |
| ttl\_sec | uint32 | Time to live of cache data in second. | Functionality | Yes | 600 (10 mins) |

### I/O

| Data name | Configuration option name | Type | Data Type | Optional |
|:--|:--|:--:|:--|:--:|
| Reusing data | cache\_key | Output | interface{} | No |

### Error

| Result code | Error reason |
|:--|:--|
| ResultInternalServerError | failed to read or write cache data |

### Dedicated statistics indicator

No any indicators exposed.

## Simple Common Mock plugin

Plugin mock a data for a failure request.

### Configuration

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--|
| plugin\_name | string | The plugin instance name. | Functionality | No | N/A |
| plugin\_concerned | string | Plugin processing failure will be considered to apply mock. | Functionality | No | N/A |
| task\_error\_code\_concerned | string | What result code will be considered to apply mock. | Functionality | No | N/A |
| mock\_task\_data\_key | string | The key name of mock data to store as the plugin output. | I/O | No | N/A |
| mock\_task\_data\_value | string | The mock data to store as the plugin output. | I/O | Yes | "" |

Available task error result code:

* ResultUnknownError
* ResultServiceUnavailable
* ResultInternalServerError
* ResultTaskCancelled
* ResultMissingInput
* ResultBadInput
* ResultRequesterGone
* ResultFlowControl

### I/O

| Data name | Configuration option name | Type | Data Type | Optional |
|:--|:--|:--:|:--|:--:|
| The key to store mock data | mock\_task\_data\_key | Output | string | No |
| Mock data | mock\_task\_data\_value | Output | string | Yes |

### Error

No any errors returned.

### Dedicated statistics indicator

No any indicators exposed.
