# Ease Gateway Plugin Development Guide

## Overview
Plugin is considered a kind of essential element managed and scheduled by Pipeline to handle corresponding parts of tasks. It aims to provide a consistent and equivalent abstraction(constructed to instances) to diverse schedulers(pipelines in Ease Gateway). It usually works with these kinds of abstraction: Pipeline, Task, PipelineContext, PipelineStatistic.

As an introduction, consider the whole model as unix pipeline, a plugin is analogous to a utility of unix and a plugin instance is analogous to a running process of a utility. so this command line
```shell
$ cat hello.txt | wc -l
```
could change to Ease Gateway form:
```shell
$ file-reader.hello | text-counter.lines
```

But there is much difference between pipeline of unix and Ease Gateway. Here is a complex example to dive into it.
```shell
> http_input.logs | io_reader.logs | json_validator.logs | http_output.logs
```
We can use this pipeline to protect backend from crashing because of too much illegal logs.  We can describe it from the administrator's perspective:
1. `http_input.logs`  waits for valid requests then generates diverse inputs like HTTP URL, HTTP Method, HTTP Version, HTTP Headers, HTTP Body
2.  `io_reader.logs`  reads HTTP Body then put it into a buffer
3.  `json_validator.logs` validates the buffer
4.  `http_output.logs` pushes it to backend

In the example above, the abstraction Task is designed to save all information within one process, we can review it from Task's perspective:
	1. Put HTTP Body in a place(KEY\_HTTP\_BODY) in the task
	2. Read a place(KEY\_HTTP\_BODY)  to another place(KEY\_JSON\_DATA) in the task - if there is no data or wrong type in the source then set error for the task
	3. Validate data in a place(KEY\_JSON\_DATA) in the task - if the data is invalid json then set error for the task
	4. Post data in a place(KEY\_JSON\_DATA) in the task as HTTP Body - if the response is unexpected then set error for the task

You can see all plugins just care their own jobs, and it just need the user to predefine interface between them. And the pipeline creates a new blank task for every new process.  the task will be stopped by pipeline at any plugins which consider the task is invalid for itself, then destroyed by pipeline or recovered by some plugins.

Now consider if the `json-validator.logs` needs to save all successful and failed number of validation, the simple but wrong method is to save them in fields of plugin struct. But here comes a problem, if user updates some configs like json schema, the numbers will be destroyed because the pipeline will reconstruct `json_validator.logs` into a new plugin instance . In this case, the life-cycle of total numbers should be the same with plugin(with a same name) not plugin instance. That's the main goal of the `PipelineContextDataBucket` we design. We can save all plugin-life-cycle data into a `PipelineContextDataBucket`. Moreover, we design the `PipelineStatistics` which provides a uniform API for looking up some critical information in Pipeline or Plugin.

So the whole flow is: Pipeline passes PipelineContext to each plugin to prepare plugin-life-cycle data and register indicators if necessary, then creates a whole new task passed into first plugin instance then passes the returned task to next plugin and so on. And it's pipeline's duty to interrupt the flow if any plugins set task error, and reconstruct plugins because some reasons for example, a plugin is updated by the administrator or appears, or appears some errors it can not swallow by itself.

Finally we can collect our abstractions together:  Pipeline is abstraction of process scheduler, Plugin is abstraction of process, Task is abstraction of one-off data stream through processes, PipelineContext is abstraction of store for process.

## Plugin
Plugin is an interface in golang:
```golang
type Plugin interface {
	// Prepare some disposable operations in PipelineContext
	Prepare(ctx pipelines.PipelineContext)
	// Run main process for each task with different PipelineContext
	Run(ctx pipelines.PipelineContext, t task.Task) (task.Task, error)
	// Name return plugin name
	Name() string
	// Close cleans plugin-instance-life-cycle resources
	Close() // the main goal of `Close` is to clean plugin-instance-life-cycle resources.
}
```

Every plugin needs to define its own `constructor function` invoked by pipeline to construct instances. Even the pipeline guarantee correct type of passed parameters, it's safer to check whether the type of argument is expected config.

### Config
Plugin config needs to inherit plugins.CommonConfig which defines essential fields.  And it should hide intermediate products such as json schema object in `json_validator` .
Just as Plugin itself, a correct config needs a `config constructor function` returning a config with default fields, and defines method `Prepare` to validator final config or initialize all intermediate products if necessary.

### Rules
* Plugin Should be implemented as stateless and be re-entry-able (idempotency) on the same task, a plugin instance could be used in different pipeline or parallel running instances of same pipeline. So do NOT update data in plugin config and plugin instance itself after construction, it will cause race condition. Under current implementation, a plugin couldn't be used in different pipeline but there is no guarantee this limitation is existing in future release.
* Function `Prepare(pipelines.PipelineContext)` guarantees it will be called on the same pipeline context against the same plugin instance only once before executing `Run(task.Task)` on the pipeline.
*  `Run(task.Task)` method should update task if an error is caused by user input, and returns error only if
   * the plugin needs reconstruction, e.g. backend failure causes local client object invalidation
   * the task has been cancelled by pipeline after running plugin is updated dynamically, task will re-run on updated plugin.
* Plugin should not do anything which might block pipeline execution in it registered recovery function, e.g. any I/O operation.

### Store of Data
A plugin might operate data in different life-cycle, here is guideline:
1. Task-life-cycle: Put data into task with `task.Value`, e.g HTTP Body in `http_input`.
2. Plugin-instance-life-cycle: Put read-only relevant data into fields of plugin/plugin-config, e.g. Schema Object generated by schema string in `json_validator` . But please put write/read data into PipelineContextDataBucket with unique instanceId which usually is memory address of the instance, e.g. successful and failed number of validation of `json_validator`. It's  `Close`'s duty to clean these resources.
3. Plugin-life-cycle: Put data into data bucket with empty instanceId. It's PipelineContext's duty to clean these resources.

## Task
Task is a finite state machine:
```
                  Pending
                     +
                     |
                     v
                  Running
                     +
                     |
         +-----------------------+
         v                       v
ResponseImmediately          Finishing
         +                       +
         +-----------------------+
                     |
                     v
                  Finished
```

Task is an interface of golang:
```golang
type Task interface {
	// Finish sets status to `Finishing`
	Finish()
	// returns flag representing finished
	Finished() bool
	// ResultCode returns current result code
	ResultCode() TaskResultCode
	// Status returns current task status
	Status() TaskStatus
	// SetError sets error message, result code, and status to `ResponseImmediately`
	SetError(err error, resultCode TaskResultCode)
	// Error returns error message
	Error() error
	// StartAt returns task start time
	StartAt() time.Time
	// FinishAt return task finish time
	FinishAt() time.Time
	// AddFinishedCallback adds callback function executing after task status set to Finished
	AddFinishedCallback(name string, callback TaskFinished) TaskFinished
	// DeleteFinishedCallback deletes registered Finished callback function
	DeleteFinishedCallback(name string) TaskFinished
	// AddRecoveryFunc adds callback function executing after task status set to `ResponseImmediately`,
	// after executing them the status of task will be recovered to `Running`
	AddRecoveryFunc(name string, taskRecovery TaskRecovery) TaskRecovery
	// DeleteRecoveryFunc deletes registered recovery function
	DeleteRecoveryFunc(name string) TaskRecovery
	// Value saves task-life-cycle value, key must be comparable
	Value(key interface{}) interface{}
	// Cancel returns a cannel channel which could be closed to broadcast cancellation of task,
	// if a plugin needs relatively long time to wait I/O or anything else,
	// it should listen this channel to exit current plugin instance.
	Cancel() <-chan struct{}
	// CancelCause returns error message of cancellation
	CancelCause() error
	// Deadline returns deadline of task if the boolean flag set true, or it's not a task with deadline cancellation
	Deadline() (time.Time, bool)
}
```

## PipelineContext
PipelineContext is an interface of golang:
```golang
type PipelineContext interface {
	// PipelineName returns pipeline name
	PipelineName() string
	// PluginNames returns sequential plugin names
	PluginNames() []string
	// Parallelism returns number of parallelism
	Parallelism() uint16
	// Statistics returns pipeline statistics
	Statistics() PipelineStatistics
	// DataBucket returns(creates a new one if necessary) pipeline data bucket corresponding with plugin.
	// If the pluginInstanceId is not empty (usually memory address of the instance),
	// the data bucket will be deleted automatically when closing the instance.
	// But if the pluginInstanceId is empty which indicates all instances of a plugin share one data bucket,
	// the data bucket won't be deleted automatically until the plugin(not plugin instance) is deleted.
	DataBucket(pluginName, pluginInstanceId string) PipelineContextDataBucket
	// DeleteBucket deletes a data bucket.
	DeleteBucket(pluginName, pluginInstanceId string) PipelineContextDataBucket
	// PreparePlugin is a hook method to invoke Prepare of plugin
	PreparePlugin(pluginName string, fun PluginPreparationFunc)
	// Close closes a PipelineContext
	Close()
}
```

### PipelineContextDataBucket
PipelineContextDataBucket is an interface in golang:
```golang
type PipelineContextDataBucket interface {
	// BindData binds data, the type of key must be comparable
	BindData(key, value interface{}) (interface{}, error)
	// QueryData querys data, return nil if not found
	QueryData(key interface{}) interface{}
	// QueryDataWithBindDefault queries data with binding default data if not found, return final value
	QueryDataWithBindDefault(key interface{}, defaultValueFunc DefaultValueFunc) (interface{}, error)
	// UnbindData unbinds data
	UnbindData(key interface{}) interface{}
}
```

## PipelineStatistics
PipelineStatistics is an interface of golang:
```golang
type PipelineStatistics interface {
	PipelineThroughputRate1() (float64, error)
	PipelineThroughputRate5() (float64, error)
	PipelineThroughputRate15() (float64, error)
	PipelineExecutionCount() (int64, error)
	PipelineExecutionTimeMax() (int64, error)
	PipelineExecutionTimeMin() (int64, error)
	PipelineExecutionTimePercentile(percentile float64) (float64, error)
	PipelineExecutionTimeStdDev() (float64, error)
	PipelineExecutionTimeVariance() (float64, error)
	PipelineExecutionTimeSum() (int64, error)

	PluginThroughputRate1(pluginName string, kind StatisticsKind) (float64, error)
	PluginThroughputRate5(pluginName string, kind StatisticsKind) (float64, error)
	PluginThroughputRate15(pluginName string, kind StatisticsKind) (float64, error)
	PluginExecutionCount(pluginName string, kind StatisticsKind) (int64, error)
	PluginExecutionTimeMax(pluginName string, kind StatisticsKind) (int64, error)
	PluginExecutionTimeMin(pluginName string, kind StatisticsKind) (int64, error)
	PluginExecutionTimePercentile(
		pluginName string, kind StatisticsKind, percentile float64) (float64, error)
	PluginExecutionTimeStdDev(pluginName string, kind StatisticsKind) (float64, error)
	PluginExecutionTimeVariance(pluginName string, kind StatisticsKind) (float64, error)
	PluginExecutionTimeSum(pluginName string, kind StatisticsKind) (int64, error)

	TaskExecutionCount(kind StatisticsKind) (uint64, error)

	PipelineIndicatorNames() []string
	PipelineIndicatorValue(indicatorName string) (interface{}, error)
	PluginIndicatorNames(pluginName string) []string
	PluginIndicatorValue(pluginName, indicatorName string) (interface{}, error)
	TaskIndicatorNames() []string
	TaskIndicatorValue(indicatorName string) (interface{}, error)

	AddPipelineThroughputRateUpdatedCallback(name string, callback PipelineThroughputRateUpdated,
		overwrite bool) (PipelineThroughputRateUpdated, bool)
	DeletePipelineThroughputRateUpdatedCallback(name string)
	DeletePipelineThroughputRateUpdatedCallbackAfterPluginDelete(name string, pluginName string)
	DeletePipelineThroughputRateUpdatedCallbackAfterPluginUpdate(name string, pluginName string)
	AddPipelineExecutionSampleUpdatedCallback(name string, callback PipelineExecutionSampleUpdated,
		overwrite bool) (PipelineExecutionSampleUpdated, bool)
	DeletePipelineExecutionSampleUpdatedCallback(name string)
	DeletePipelineExecutionSampleUpdatedCallbackAfterPluginDelete(name string, pluginName string)
	DeletePipelineExecutionSampleUpdatedCallbackAfterPluginUpdate(name string, pluginName string)
	AddPluginThroughputRateUpdatedCallback(name string, callback PluginThroughputRateUpdated,
		overwrite bool) (PluginThroughputRateUpdated, bool)
	DeletePluginThroughputRateUpdatedCallback(name string)
	DeletePluginThroughputRateUpdatedCallbackAfterPluginDelete(name string, pluginName string)
	DeletePluginThroughputRateUpdatedCallbackAfterPluginUpdate(name string, pluginName string)
	AddPluginExecutionSampleUpdatedCallback(name string, callback PluginExecutionSampleUpdated,
		overwrite bool) (PluginExecutionSampleUpdated, bool)
	DeletePluginExecutionSampleUpdatedCallback(name string)
	DeletePluginExecutionSampleUpdatedCallbackAfterPluginDelete(name string, pluginName string)
	DeletePluginExecutionSampleUpdatedCallbackAfterPluginUpdate(name string, pluginName string)

	RegisterPluginIndicator(pluginName, pluginInstanceId, indicatorName, desc string,
		evaluator StatisticsIndicatorEvaluator) (bool, error)
	UnregisterPluginIndicator(pluginName, pluginInstanceId, indicatorName string)
	UnregisterPluginIndicatorAfterPluginDelete(pluginName, pluginInstanceId, indicatorName string)
	UnregisterPluginIndicatorAfterPluginUpdate(pluginName, pluginInstanceId, indicatorName string)
}
```

## Notice

