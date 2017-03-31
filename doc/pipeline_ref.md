Ease Gateway Pipeline Reference
=================================

## Pipeline Content

There is 1 available pipeline in Ease Gateway current release.

| Pipeline name | Type name | Functional | Development status | Link |
|:--|:--|:--:|:--:|:--:|
| [Linear Pipeline](#linear-pipeline) | Linear | Yes | GA | [code](https://github.com/hexdecteam/easegateway/blob/master/src/model/linear.go) |


## Linear Pipeline

Linear Pipeline is a model to define a unidirectional path plugins handling task parallelly(not concurrently).

### Configuration

| Parameter name | Data type (golang) | Description | Type | Optional | Default value (golang) |
|:--|:--|:--|:--:|:--:|:--:|
| pipeline\_name | string | The pipeline instance name. | Functionality | No | N/A |
| plugin\_names | []string | The sequential list of plugins handling tasks. | Functionality | No | N/A |
| parallelism | uint16 | The parallel number of linear pipleine. |Functionality | Yes | 1 |
| wait\_plugin\_close | bool | The flag represents if the pipeline suspends until the rotten plugins closed. | Functionality | Yes | true |

> Notice: The reason why `wait_plugin_close` appears is to  prevent that a new instance can not complete construction because a old instance(same plugin) is still holding some critical and unique resources. e.g. A instance of http\_input.metrics with holding url `/v1/meitrcs` suddenly is updated(with the same url `/v1/metrics`), the pipeline can not construct a new instance before the old instance closes, because the unique resource url `/v1/metrics` can not be occupied by multiple instances simultaneously.

### Dedicated statistics indicator

| Indicator name | Level | Data type (golang) | Description |
|:--|:--:|:--|:--|
| THROUGHPUT\_RATE\_LAST\_1MIN\_ALL | Pipeline | float64 | Throughtput rate of the pipeline in last 1 minute. |
| THROUGHPUT\_RATE\_LAST\_5MIN\_ALL | Pipeline | float64 | Throughtput rate of the pipeline in last 5 minute. |
| THROUGHPUT\_RATE\_LAST\_15MIN\_ALL | Pipeline | float64 | Throughtput rate of the pipeline in last 15 minute. |
| EXECUTION\_COUNT\_ALL | Pipeline | int64 | Total execution count of the pipeline. |
| EXECUTION\_TIME\_MAX\_ALL | Pipeline | int64 | Maximal time of execution time of the pipeline in nanosecond. |
| EXECUTION\_TIME\_MIN\_ALL | Pipeline | int64 | Minimal time of execution time of the pipeline in nanosecond. |
| EXECUTION\_TIME\_50\_PERCENT\_ALL | Pipeline | float64 | 50% execution time of the pipeline in nanosecond. |
| EXECUTION\_TIME\_90\_PERCENT\_ALL | Pipeline | float64 | 90% execution time of the pipeline in nanosecond. |
| EXECUTION\_TIME\_99\_PERCENT\_ALL | Pipeline | float64 | 99% execution time of the pipeline in nanosecond. |
| EXECUTION\_TIME\_STD\_DEV\_ALL | Pipeline | float64 | Standard deviation of execution time of the pipeline in nanosecond. |
| EXECUTION\_TIME\_VARIANCE\_ALL | Pipeline | float64 | Variance of execution time of the pipeline. |
| EXECUTION\_TIME\_SUM\_ALL | Pipeline | int64 | Sum of execution time of the pipeline in nanosecond. |
| THROUGHPUT\_RATE\_LAST\_1MIN\_ALL | Plugin | float64 | Throughtput rate of the plugin in last 1 minute. |
| THROUGHPUT\_RATE\_LAST\_5MIN\_ALL | Plugin | float64 | Throughtput rate of the plugin in last 5 minute. |
| THROUGHPUT\_RATE\_LAST\_15MIN\_ALL | Plugin | float64 | Throughtput rate of the plugin in last 15 minute. |
| THROUGHPUT\_RATE\_LAST\_1MIN\_SUCCESS | Plugin | float64 | Successful throughtput rate of the plugin in last 1 minute. |
| THROUGHPUT\_RATE\_LAST\_5MIN\_SUCCESS | Plugin | float64 | Successful throughtput rate of the plugin in last 5 minute. |
| THROUGHPUT\_RATE\_LAST\_15MIN\_SUCCESS | Plugin | float64 | Successful throughtput rate of the plugin in last 15 minute. |
| THROUGHPUT\_RATE\_LAST\_1MIN\_FAILURE | Plugin | float64 | Failed throughtput rate of the plugin in last 1 minute. |
| THROUGHPUT\_RATE\_LAST\_5MIN\_FAILURE | Plugin | float64 | Failed throughtput rate of the plugin in last 5 minute. |
| THROUGHPUT\_RATE\_LAST\_15MIN\_FAILURE | Plugin | float64 | Failed throughtput rate of the plugin in last 15 minute. |
| EXECUTION\_COUNT\_ALL | Plugin | int64 | Total execution count of the plugin. |
| EXECUTION\_COUNT\_SUCCESS | Plugin | int64 | Successful execution count of the plugin. |
| EXECUTION\_COUNT\_FAILURE | Plugin | int64 | Failed execution count of the plugin. |
| EXECUTION\_TIME\_MAX\_ALL | Plugin | int64 | Maximal time of execution time of the plugin in nanosecond. |
| EXECUTION\_TIME\_MAX\_SUCCESS | Plugin | int64 | Maximal time of successful execution of the plugin in nanosecond. |
| EXECUTION\_TIME\_MAX\_FAILURE | Plugin | int64 | Maximal time of failure execution of the plugin in nanosecond. |
| EXECUTION\_TIME\_MIN\_ALL | Plugin | int64 | Minimal time of execution time of the plugin in nanosecond. |
| EXECUTION\_TIME\_MIN\_SUCCESS | Plugin | int64 | Minimal time of successful execution of the plugin in nanosecond. |
| EXECUTION\_TIME\_MIN\_FAILURE | Plugin | int64 | Minimal time of failure execution of the plugin in nanosecond. |
| EXECUTION\_TIME\_50\_PERCENT\_SUCCESS | Plugin | float64 | 50% successful execution time of the plugin in nanosecond. |
| EXECUTION\_TIME\_50\_PERCENT\_FAILURE | Plugin | float64 | 50% failure execution time of the plugin in nanosecond. |
| EXECUTION\_TIME\_90\_PERCENT\_SUCCESS | Plugin | float64 | 90% successful execution time of the plugin in nanosecond. |
| EXECUTION\_TIME\_90\_PERCENT\_FAILURE | Plugin | float64 | 90% failure execution time of the plugin in nanosecond. |
| EXECUTION\_TIME\_99\_PERCENT\_SUCCESS | Plugin | float64 | 99% successful execution time of the plugin in nanosecond. |
| EXECUTION\_TIME\_99\_PERCENT\_FAILURE | Plugin | float64 | 99% failure execution time of the plugin in nanosecond. |
| EXECUTION\_TIME\_STD\_DEV\_SUCCESS | Plugin | float64 | Standard deviation of successful execution time of the plugin in nanosecond. |
| EXECUTION\_TIME\_STD\_DEV\_FAILURE | Plugin | float64 | Standard deviation of failure execution time of the plugin in nanosecond. |
| EXECUTION\_TIME\_VARIANCE\_SUCCESS | Plugin | float64 | Variance of successful execution time of the plugin. |
| EXECUTION\_TIME\_VARIANCE\_FAILURE | Plugin | float64 | Variance of failure execution time of the plugin. |
| EXECUTION\_TIME\_SUM\_ALL | Plugin | int64 |  Sum of execution time of the plugin in nanosecond. |
| EXECUTION\_TIME\_SUM\_SUCCESS | Plugin | int64 | Sum of successful execution time of the plugin in nanosecond. |
| EXECUTION\_TIME\_SUM\_FAILURE | Plugin | int64 | Sum of failure execution time of the plugin in nanosecond. |
| EXECUTION\_COUNT\_ALL | Task | uint64 | Total task execution count. |
| EXECUTION\_COUNT\_SUCCESS | Task | uint64 | Successful task execution count. |
| EXECUTION\_COUNT\_FAILURE | Task | uint64 | Failed task execution count. |
