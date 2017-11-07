Ease Gateway Plugin Reference
===============================

## Summary

There are 20 built-in plugins totally in Ease Gateway current release.

| Plugin name | Type name | Block-able | Functional | Development status | Link |
|:--|:--|:--:|:--:|:--:|:--|
| [HTTP input](#http-input-plugin) | HTTPInput | Yes | Yes | GA | [code](../src/plugins/http_input.go) |
| [JSON validator](#json-validator-plugin) | JSONValidator | No | Yes | GA | [code](../src/plugins/json_validator.go) |
| [Kafka output](#kafka-output-plugin) | KafkaOutput | No | Yes | GA | [code](../src/plugins/kafka_output.go) |
| [HTTP output](#http-output-plugin) | HTTPOutput | No | Yes  | GA | [code](../src/plugins/http_output.go) |
| [IO reader](#io-reader-plugin) | IOReader | Yes | Yes | GA | [code](../src/plugins/io_reader.go) |
| [HTTP header counter](#http-header-counter-plugin) | HTTPHeaderCounter | No | No | GA | [code](../src/plugins/http_header_counter.go) |
| [Throughput rate limiter](#throughput-rate-limiter-plugin) | ThroughputRateLimiter | Yes | No | GA | [code](../src/plugins/throughput_rate_limiter.go) |
| [Latency limiter](#latency-limiter-plugin) | LatencyWindowLimiter | Yes | No | GA | [code](../src/plugins/latency_limiter.go) |
| [Service circuit breaker](#service-circuit-breaker-plugin) | ServiceCircuitBreaker | No | No | GA | [code](../src/plugins/service_circuit_breaker.go) |
| [Static pass probability limiter](#static-pass-probability-limiter-plugin) | StaticProbabilityLimiter | No | No | GA | [code](../src/plugins/static_probability_limiter.go) |
| [No more failure limiter](#no-more-failure-limiter-plugin) | NoMoreFailureLimiter | No | No | GA | [code](../src/plugins/no_more_failure_limiter.go) |
| [Simple common cache](#simple-common-cache-plugin) | SimpleCommonCache | No | No | GA | [code](../src/plugins/simple_common_cache.go) |
| [Simple common mock](#simple-common-mock-plugin) | SimpleCommonMock | No | No | GA | [code](../src/plugins/simple_common_mock.go) |
| [Python](#python-plugin) | Python | Yes | Yes | GA | [code](../src/plugins/python.go) |
| [Shell](#shell-plugin) | Shell | Yes | Yes | GA | [code](../src/plugins/shell.go) |
| [Upstream output](#upstream-output-plugin) | UpstreamOutput | Yes | No | GA | [code](../src/plugins/upstream_output.go) |
| [Downstream input](#downstream-input-plugin) | DownstreamInput | Yes | No | GA | [code](../src/plugins/downstream_input.go) |
| [Ease Monitor graphite validator](#ease-monitor-graphite-validator-plugin) | EaseMonitorGraphiteValidator | No | Yes | GA | [code](../src/plugins/easemonitor/graphite_validator.go) |
| [Ease Monitor graphite GID extractor](#ease-monitor-graphite-gid-extractor-plugin) | EaseMonitorGraphiteGidExtractor | No | Yes | GA | [code](../src/plugins/easemonitor/graphite_gid_extractor.go) |
| [Ease Monitor JSON GID extractor](#ease-monitor-json-gid-extractor-plugin) | EaseMonitorJSONGidExtractor | No | Yes | GA | [code](../src/plugins/easemonitor/json_gid_extractor.go) |


## HTTP Input plugin

Plugin handles HTTP request and returns client with pipeline processed response. Currently a HTTPS server will runs on a fixed 10443 port with a certificate and key file pair.

### Configuration

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--|
| plugin\_name | string | The plugin instance name. | Functionality | No | N/A |
| url | string | The request HTTP url plugin will proceed. Parametric url is supported, like `/user/{user}` or `/{resource}/count`, the values of each parameter will be extracted into task. We do not allow to register static path and parametric path on the same segment, e.g. you can not register the patterns `/user/jack` and `/user/{user}` on the same http method. | Functionality | No | N/A |
| methods | []string | The request HTTP methods plugin will proceed. | Functionality | Yes | {"GET"} |
| headers\_enum | map\[string\][]string | The request HTTP headers plugin will proceed. | Functionality | Yes | nil |
| unzip | bool | The flag represents if the plugin decompresses the request body when request content is encoded in GZIP. | Functionality | Yes | true |
| respond\_error | bool | The flag represents if the plugin respond error information to client if pipeline handles the request unsuccessfully. The option will be used only when `response_body_io_key` and `response_body_buffer_key` options are empty. | Functionality | Yes | false |
| fast\_close | bool | The flag represents if the plugin does not wait any response which is processing before close, e.g. ignore data transmission on a slow connection. | Functionality | Yes | false |
| dump\_request | string | The flag represents if the plugin dumps http request out to `http_dump.log` log file (exclude body). Three options are available `auto`, `yes`/`y`/`true`/`t`/`on`/`1` or `no`/`n`/`false`/`f`/`off`/`0` (case-insensitive). In `auto` option, the request will be dumped only when the gateway running on `debug` or `test` stage. | Functionality | Yes | "auto" |
| request\_header\_names\_key | string | The name of HTTP request header name list stored in internal storage as the plugin output. | I/O | Yes | "" |
| request\_body\_io\_key | string | The key name of HTTP request body io object stored in internal storage as the plugin output. | I/O | Yes | "" |
| response\_code\_key | string | The key name of HTTP response status code value stored in internal storage as the plugin input. An empty value of the option means returning pipeline handling result code to client. | I/O | Yes | "" |
| response\_body\_io\_key | string | The key name of HTTP response body io object stored in internal storage as the plugin input. | I/O | Yes | "" |
| response\_body\_buffer\_key | string | The key name of HTTP response body buffer stored in internal storage as the plugin input. The option will be leveraged only when `response_body_io_key` option is empty. | I/O | Yes | "" |
| response\_remote\_key | string | The key name of HTTP response remote address stored in internal storage as the plugin input. | I/O | Yes | "" |
| response\_duration\_key | string | The key name of HTTP response process time stored in internal storage as the plugin input. | I/O | Yes | "" |

### I/O

| Data name | Configuration option name | Type | Data Type | Optional |
|:--|:--|:--:|:--|:--:|
| Each parameter value of parametric request url | N/A | Output | string | N/A |
| Each [CGI environment value](#https://tools.ietf.org/html/rfc3875#section-4.1) of the request | N/A | Output | string | N/A |
| Request header name list | request\_header\_names\_key | Output | []string | Yes |
| Request body IO object | request\_body\_io\_key | Output | io.Reader | Yes |
| Response HTTP status code | response\_code\_key | Input | int | Yes |
| Response body IO object | response\_body\_io\_key | Input | io.Reader | Yes |
| Response body buffer | response\_body\_buffer\_key | Input | []byte | Yes |
| Response remote address | response\_remote\_key | Input | string | Yes |
| Response process time | response\_duration\_key | Input | time.Duration | Yes |

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

## JSON Validator plugin

Plugin validates input data, to check if it's a valid json data with a special schema.

### Configuration

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--|
| plugin\_name | string | The plugin instance name. | Functionality | No | N/A |
| schema | string | The schema json data needs to accord. | Functionality | No | N/A |
| base64\_encoded | bool | The flag represents if the schema is encoded in base64. | Functionality | Yes | false |
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

## Kafka Output plugin

Plugin outputs request data to a kafka service.

### Configuration

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--|
| plugin\_name | string | The plugin instance name. | Functionality | No | N/A |
| brokers | []string | The kafka broker list. | Functionality | No | N/A |
| client\_id | string | The client id as a kafka consumer. | Functionality | Yes | "easegateway" |
| topic | string | The topic data outputs to. | Functionality | No | N/A |
| message\_key\_key | string |The key name of message key value stored in internal storage as the plugin input. | I/O | Yes | "" |
| data\_key | string | The key name of message data stored in internal storage as the plugin input. | I/O | No | N/A |

### I/O

| Data name | Configuration option name | Type | Data Type | Optional |
|:--|:--|:--:|:--|:--:|
| Message key | message\_key\_key | Input | string | Yes |
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

## HTTP Output plugin

Plugin outputs request data to a HTTP endpoint.

### Configuration

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--|
| plugin\_name | string | The plugin instance name. | Functionality | No | N/A |
| url\_pattern | string | The pattern of the complete HTTP output endpoint. E.g. ``https://1.2.3.4/abc?def={INPUT_DATA}`` | Functionality | No | N/A |
| header\_patterns | map\[string\]string | The list of HTTP output header name pattern and value pattern pair. | Functionality | Yes | nil |
| method | string | The method HTTP output used. | Functionality | No | N/A |
| expected\_response\_codes| []int | The expected HTTP response status code. If HTTPOutput doesn't need to check the response status code, then set it to an empty array. Else if the real HTTP response status code doesn't match any of them, then the pipeline is finished. | Functionality | Yes | []{http.StatusOK} |
| timeout\_sec | uint16 | The request timeout HTTP output limited in second. | Functionality | Yes | 120 (2 minutes) |
| cert\_file | string | The certificate file name HTTPS output used. The option is mandatory if HTTPS is expected as output protocol. | Functionality | Yes | "" |
| key\_file | string | The key file name HTTPS output used. The option is mandatory if HTTPS is expected as output protocol. | Functionality | Yes | "" |
| ca\_file | string | The root certificate file name HTTPS output used. | Functionality | Yes | "" |
| insecure\_tls | bool | The flag represents if the plugin does not check server certificate. | Functionality | Yes | false |
| close\_body\_after\_pipeline | bool | The flag represents if to close the http body IO object after task finished the pipeline. | Functionality | Yes | false |
| keepalive | string | The flag represents if the plugin configures keep-alive for an active connection. Three options are available `auto`, `yes`/`y`/`true`/`t`/`on`/`1` or `no`/`n`/`false`/`f`/`off`/`0` (case-insensitive). In `auto` option, the keep-alive feature will be applied according to the original request from http input plugin for http proxy case. | Functionality | Yes | "auto" |
| keepalive\_sec | string | The flag specifies the keep-alive period for an active network connection. Network protocols that do not support keep-alive ignore this option. | Functionality | Yes | 30 |
| dump\_request | string | The flag represents if the plugin dumps http request out to `http_dump.log` log file (exclude body). Three options are available `auto`, `yes`/`y`/`true`/`t`/`on`/`1` or `no`/`n`/`false`/`f`/`off`/`0` (case-insensitive). In `auto` option, the request will be dumped only when the gateway running on `debug` or `test` stage. | Functionality | Yes | "auto" |
| dump\_response | string | The flag represents if the plugin dumps http response out to `http_dump.log` log file (exclude body). Three options are available `auto`, `yes`/`y`/`true`/`t`/`on`/`1` or `no`/`n`/`false`/`f`/`off`/`0` (case-insensitive). In `auto` option, the response will be dumped only when the gateway running on `debug` or `test` stage. | Functionality | Yes | "auto" |
| request\_body\_buffer\_pattern | string | The HTTP output body buffer pattern. The option will be leveraged only when `request_body_io_key` option is empty. | Functionality | Yes | "" |
| request\_body\_io\_key | string | The HTTP output body io object. | I/O | Yes | "" |
| response\_code\_key | string | The key name of HTTP response status code value stored in internal storage as the plugin output. An empty value of the option means the plugin does not output HTTP response status code. | I/O | Yes | "" |
| response\_body\_io\_key | string | The key name of HTTP response body io object stored in internal storage as the plugin output. An empty value of the option means the plugin does not output HTTP response body io object.| I/O | Yes | "" |
| response\_remote\_key | string | The key name of HTTP response remote address stored in internal storage as the plugin input. An empty value of the option means the plugin does not output HTTP reponse remote address. | I/O | Yes | "" |
| response\_duration\_key | string | The key name of HTTP response process time stored in internal storage as the plugin output. An empty value of the option means the plugin does not output HTTP response process time. | I/O | Yes | "" |

### I/O

| Data name | Configuration option name | Type | Data Type | Optional |
|:--|:--|:--:|:--|:--:|
| Request HTTP body | request\_body\_io\_key | Input | io.Reader | Yes |
| Response HTTP status code | response\_code\_key | Output | int | Yes |
| Response body IO object | response\_body\_io\_key | Output | io.ReadCloser | Yes |
| Response remote address | response\_remote\_key | Output | string | Yes |
| Response process time | response\_duration\_key | Output | time.Duration | Yes |

> NOTE:
> Kindly reminder for plugin developer: if the option `response_body_io_key` is configured, downstream plugin has the responsibility to close it after use to prevent resource leak.

### Error

| Result code | Error reason |
|:--|:--|
| ResultServiceUnavailable | failed to send HTTP request |
| ResultInternalServerError | failed to create HTTP request |
| ResultInternalServerError | failed to output response HTTP status code |
| ResultInternalServerError | failed to output response body IO object |
| ResultInternalServerError | failed to output remote address of the response |
| ResultInternalServerError | failed to output process time of the response |

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
| base64\_coding | string | The flag represents if to encode or decode the output read from IO object. Valid flag are `` (empty), `encode` and `decode`. | Functionality | Yes | "" |
| input\_key | string | The key name of IO object stored in internal storage as the plugin input. | I/O | No | N/A |
| output\_key | string | The key name of read out data as the plugin output. | I/O | No | N/A |

### I/O

| Data name | Configuration option name | Type | Data Type | Optional |
|:--|:--|:--:|:--|:--:|
| IO object to read | input\_key | Input | io.Reader, io.ReadCloser | No |
| Data buffer read out | output\_key | Output | []byte | No |

### Error

| Result code | Error reason |
|:--|:--|
| ResultMissingInput | input got wrong value |
| ResultBadInput | failed to read data |
| ResultBadInput | failed to encode or decode data as base64 coding |
| ResultInternalServerError | failed to output data buffer |

### Dedicated statistics indicator

No any indicators exposed.

## HTTP Header Counter plugin

Plugin calculates request count in a recent period which has a special header name. This behavior likes count session amount.

### Configuration

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--|
| plugin\_name | string | The plugin instance name. | Functionality | No | N/A |
| header\_concerned | string | The header name plugin calculates. | Functionality | No | N/A |
| expiration\_sec | uint32 | The recent period in second. | Functionality | No | N/A |

### I/O

No any inputs or outputs.

### Error

| Result code | Error reason |
|:--|:--|
| ResultMissingInput | input got wrong value |

### Dedicated statistics indicator

| Indicator name | Data type (golang) | Description |
|:--:|:--:|:--|
| RECENT\_HEADER\_COUNT | uint64 | The count of HTTP requests that the header of each one contains the key in the recent period. |

## Throughput Rate Limiter plugin

Plugin limits request rate based on current throughput.

### Configuration

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--|
| plugin\_name | string | The plugin instance name. | Functionality | No | N/A |
| tps | string| The maximal requests per second. Value -1 means no limitation. Value zero here means there is no request could be processed.| Functionality | No | N/A |
| timeout\_msec | int64 | The maximal wait time in millisecond of request queuing. Value 0 means there is no request could be queued, value -1 means there is no queuing timeout.| Functionality | Yes | 200 |

### I/O

No any inputs or outputs.

### Error

| Result code | Error reason |
|:--|:--|
| ResultFlowControl | service is unavailable caused by throughput rate limit |
| ResultTaskCancelled | task is cancelled |

### Dedicated statistics indicator

No any indicators exposed.

## Latency Limiter plugin

Plugin limits request rate based on historical latency statistics.

### Configuration

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--|
| plugin\_name | string | The plugin instance name. | Functionality | No | N/A |
| plugins\_concerned | []string | Plugin name list, the execution time on 90% requests of each of them will be calculated, the result will be compared with the `latency_threshold_msec` option. | Functionality | No | N/A |
| latency\_threshold\_msec | uint32 | The latency threshold in millisecond, when the latency is greater than it the request will be counted as one slow request. Normally, this value could be 1.5 * average-normal-response-time. | Functionality | Yes | 800 |
| backoff\_msec | uint16 | How many milliseconds the request need to be delayed when the times of the slow request reached. Value 0 means there is not request could be delayed, respond StatusTooManyRequests immediately. Normally, this value could be 0.5 * average-normal-response-time. | Functionality | Yes | 100 |
| allow\_times | uint32 | Allowed times of slow request before to perform backoff. | Functionality | Yes | 0 |


### I/O

No any inputs or outputs.

### Error

| Result code | Error reason |
|:--|:--|
| ResultFlowControl | service is unavailable caused by latency limit |
| ResultTaskCancelled | task is cancelled |

### Dedicated statistics indicator

No any indicators exposed.

## Service Circuit Breaker plugin

Plugin limits request rate base on the failure rate of one or more plugins.

### Configuration

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--|
| plugin\_name | string | The plugin instance name. | Functionality | No | N/A |
| plugins\_concerned | []string | Plugins their processing failure will be considered to control circuit breaker status. | Functionality | No | N/A |
| all\_tps\_threshold\_to\_enable | float64 | As the condition, it indicates how many requests per second will cause circuit breaker to be enabled. Value zero means to enable circuit breaker immediately when a request arrived. | Functionality | Yes | 1 |
| failure\_tps\_threshold\_to\_break | float64 | As the condition, it indicates how many failure requests per second will cause circuit breaker to be turned on. It means fully close request flow. Value zero here means breaker will keep open or half-open status. | Functionality | Yes | 1 |
| failure\_tps\_percent\_threshold\_to\_break | float32 | As the condition, it indicates what percent of failure requests per second will cause circuit breaker to be turned on. It means fully close request flow. Value zero here means breaker will keep open or half-open status. The option can be leveraged only when `failure_tps_threshold_to_break` condition does not satisfy. | Functionality | No | N/A |
| recovery\_time\_msec | uint32 | As the condition, it indicates how long delay in milliseconds will cause circuit breaker to be turned to half-open status, the status is used to try service availability. In general, it equals to MTTR. | Functionality | Yes | 1000 |
| success\_tps\_threshold\_to\_open | float64 | As the condition, it indicates how many success requests per second will cause circuit breaker to be turned off. It means fully open request flow. Value zero here means to fully open request flow immediately after recovery time elapsed. | Functionality | Yes | 1 |

### I/O

No any inputs or outputs.

### Error

| Result code | Error reason |
|:--|:--|
| ResultFlowControl | service is unavailable caused by service fusing |

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
| ResultFlowControl | service is unavailable caused by probability limit |

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
| ResultFlowControl | service is unavailable caused by failure limitation |

### Dedicated statistics indicator

No any indicators exposed.

## Simple Common Cache plugin

Plugin caches a data and uses it to serve follow requests directly.

### Configuration

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--|
| plugin\_name | string | The plugin instance name. | Functionality | No | N/A |
| hit\_keys | []string | All the data with every key will be considered to check if the request hits or misses the cache. | Functionality  | No | N/A|
| ttl\_sec | uint32 | Time to live of cache data in second. | Functionality | Yes | 600 (10 mins) |
| cache\_key | string | The data with the key will be cached in internal storage as the plugin input (caching) and output (reusing). | I/O | No | N/A |
| finish\_if\_hit | bool | The flag represents if the pipeline is finished after hitting cache data | Functionality | Yes | true |

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

Plugin mocks a data for a failure request.

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

## Python Plugin

Plugin executes python code.

### Configuration

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--|
| plugin\_name | string | The plugin instance name. | Functionality | No | N/A |
| code | string | The python code to be executed. | Functionality | No | N/A |
| base64\_encoded | bool | The flag represents if the python code is encoded in base64. | Functionality | Yes | false |
| version | string | The version of python interpreter. Currently valid values are `2` and `3`.  | Functionality | Yes | "2" |
| timeout_sec | uint16 | The wait timeout python code execution limited in second. | Functionality | Yes | 10 |
| input\_buffer\_pattern | string | The input data buffer pattern for python code as the input. | I/O | Yes | "" |
| output\_key | string | The key name of standard output data for python code stored in internal storage as the plugin output. | I/O | Yes | "" |
| expected\_exit\_codes| []int | The expected python code exit code. If the plugin doesn't need to check the exit code, then set it to an empty array. Else if the real exit code doesn't match any of them, then the pipeline is finished. | Functionality | Yes | []{0} |

### I/O

| Data name | Configuration option name | Type | Data Type | Optional |
|:--|:--|:--:|:--|:--:|
| Standard input data | input\_buffer\_pattern | Input | []byte | Yes |
| Standard output data | output\_key | Output | []byte | Yes |

### Error

| Result code | Error reason |
|:--|:--|
| ResultServiceUnavailable | failed to get standard input |
| ResultServiceUnavailable | failed to launch python interpreter |
| ResultServiceUnavailable | failed to execute python code |
| ResultServiceUnavailable | execute python code timeout and terminated |
| ResultInternalServerError | failed to load/read data |
| ResultTaskCancelled | task is cancelled |

### Dedicated statistics indicator

No any indicators exposed.

## Shell Plugin

Plugin executes shell script.

### Configuration

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--|
| plugin\_name | string | The plugin instance name. | Functionality | No | N/A |
| code | string | The shell script to be executed. | Functionality | No | N/A |
| base64_encoded | bool | The flag represents if the shell script is encoded in base64. | Functionality | Yes | false |
| type | string | The type of shell interpreter. Currently valid values are `sh`, `bash` and `zsh`. | Functionality | Yes | "sh" |
| timeout_sec | uint16 | The wait timeout shell script execution limited in second. | Functionality | Yes | 10 |
| input\_buffer\_pattern | string | The input data buffer pattern for shell script as the input. | I/O | Yes | "" |
| output\_key | string | The key name of standard output data for shell script stored in internal storage as the plugin output. | I/O | Yes | "" |
| expected\_exit\_codes| []int | The expected shell script exit code. If the plugin doesn't need to check the exit code, then set it to an empty array. Else if the real exit code doesn't match any of them, then the pipeline is finished. | Functionality | Yes | []{0} |

### I/O

| Data name | Configuration option name | Type | Data Type | Optional |
|:--|:--|:--:|:--|:--:|
| Standard input data | input\_buffer\_pattern | Input | []byte | Yes |
| Standard output data | output\_key | Output | []byte | Yes |

### Error

| Result code | Error reason |
|:--|:--|
| ResultServiceUnavailable | failed to get standard input |
| ResultServiceUnavailable | failed to launch shell interpreter |
| ResultServiceUnavailable | failed to execute shell script |
| ResultServiceUnavailable | execute shell script timeout and terminated |
| ResultInternalServerError | failed to load/read data |
| ResultTaskCancelled | task is cancelled |

### Dedicated statistics indicator

No any indicators exposed.

## Upstream Output plugin

Plugin outputs request to an upstream pipeline and waits the response.

### Configuration

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--|
| plugin\_name | string | The plugin instance name. | Functionality | No | N/A |
| target\_pipelines | []string | The list of upstream pipeline name. | Functionality | No | N/A |
| route\_policy | string | The name of route policy which is used to select a upstream pipeline form `target_pipelines` option for a task. Available policies are `round_robin`, `weighted_round_robin`, `random`, `weighted_random`, `least_wip_requests`, `hash`, `filter`, `fanout` and `retry`. | Functionality | Yes | "round_robin" |
| timeout\_sec | uint16 | The wait timeout upstream process limited in second. | Functionality | Yes | 120 (2 minutes) |
| request\_data\_keys | []string | The key names of the data in current pipeline, each of them will be passed to target pipeline as the input part of cross-pipeline request. Plugin `downstream_input` will handle the data as the input. | I/O | No | [] |
| target\_weights | []uint16 | The weight of each upstream pipeline, only for `weighted_round_robin` and `weighted_random` policies. | Functionality | Yes | \[1...\] |
| value\_hashed\_keys | string | The key names of the value in current pipeline which is used to calculate hash value for `hash` policy of upstream pipeline selection. | Functionality | No | N/A |
| filter\_conditions | []map\[string\]string | Each map in the list as the condition set for the target pipeline according to the index. The Map key is the key of value in the task, map value is the match condition, support regular expression. | Functionality | No | N/A |
| target\_response\_flags | []bool | The flag list to represent if handling the upstream pipeline response, only for `fanout` policy. | Functionality | Yes | \[false...\] |

### I/O

| Data name | Configuration option name | Type | Data Type | Optional |
|:--|:--|:--:|:--|:--:|
| Data of cross-pipeline request | request\_data\_keys | Output (intents to send to upstream pipeline) | map\[interface{}\]interface{} | Yes |

### Error

| Result code | Error reason |
|:--|:--|
| ResultServiceUnavailable | upstream pipeline selector returns empty pipeline name |
| ResultServiceUnavailable | upstream is timeout |
| ResultServiceUnavailable | failed to commit cross-pipeline request to upstream |
| ResultInternalServerError | downstream received nil upstream response |
| ResultInternalServerError | downstream received wrong upstream response |
| ResultInternalServerError | failed to output data |
| ResultTaskCancelled | task is cancelled |

### Dedicated statistics indicator

No any indicators exposed.

## Downstream Input plugin

Plugin handles downstream request to running pipeline as input and send the response back.

### Configuration

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--|
| plugin\_name | string | The plugin instance name. | Functionality | No | N/A |
| response\_data\_keys | []string | The key names of the data in current pipeline, each of them will be send back to downstream pipeline as the output part of cross-pipeline response. Plugin `upstream_input` will handle the data as the output. | I/O | No | [] |

### I/O

| Data name | Configuration option name | Type | Data Type | Optional |
|:--|:--|:--:|:--|:--:|
| Data of cross-pipeline response | response\_data\_keys | Input (intents to send to downstream pipeline) | map\[interface{}\]interface{} | Yes |

### Error

| Result code | Error reason |
|:--|:--|
| ResultInternalServerError | upstream received wrong downstream request |
| ResultInternalServerError | failed to output data |

### Dedicated statistics indicator

No any indicators exposed.

## Ease Monitor Graphite Validator plugin

Plugin validates input data, to check if it's a valid Ease Monitor graphite data with plaintext protocol.

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

## Ease Monitor Graphite GID Extractor plugin

Plugin extracts Ease Monitor global ID from Ease Monitor graphite data.

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

## Ease Monitor JSON GID extractor plugin

Plugin extracts Ease Monitor global ID from Ease Monitor json data.

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
