# Ease Gateway Plugin Development Guide
## Quick Start
Let's create a simplest plugin named `hello` which set a value to key `hello` in the task. This one give you a basic taste of plugin development, You can ignore confusing stuff that will be clarified soon after.

```golang
package plugins

import (
	"common"
	"fmt"

	"github.com/megaease/easegateway-types/pipelines"
	"github.com/megaease/easegateway-types/plugins"
	"github.com/megaease/easegateway-types/task"
)

type helloConfig struct {
	common.PluginCommonConfig
}

func HelloConfigConstructor() plugins.Config { return &helloConfig{} }

func (c *helloConfig) Prepare(pipelineNames []string) error {
	err := c.PluginCommonConfig.Prepare(pipelineNames)
	if err != nil {
		return err
	}

	return nil
}

type hello struct {
	conf *helloConfig
}

func HelloConstructor(conf plugins.Config) (plugins.Plugin, error) {
	c, ok := conf.(*helloConfig)
	if !ok {
		return nil, fmt.Errorf("config type want *helloConfig got %T", conf)
	}

	return &hello{
		conf: c,
	}, nil
}

func (h *hello) Prepare(ctx pipelines.PipelineContext) {}

func (h *hello) sayHello(t task.Task) (error, task.TaskResultCode, task.Task) {
	t, err := task.WithValue(t, "hello", "world")
	if err != nil {
		return err, task.ResultInternalServerError, t
	}

	return nil, t.ResultCode(), t
}

func (h *hello) Run(ctx pipelines.PipelineContext, t task.Task) (task.Task, error) {
	err, resultCode, t := h.sayHello(t)
	if err != nil {
		t.SetError(err, resultCode)
	}

	return t, nil
}

func (h *hello) Name() string { return h.conf.PluginName() }

func (h *hello) Close() {}
```

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
2. `io_reader.logs`  reads HTTP Body then put it into a buffer
3. `json_validator.logs` validates the buffer
4. `http_output.logs` pushes it to backend

In the example above, the abstraction Task is designed to save all information within one process, we can review it from Task's perspective:
1. Put HTTP Body in a place(KEY\_HTTP\_BODY) in the task
2. Read a place(KEY\_HTTP\_BODY)  to another place(KEY\_JSON\_DATA) in the task - if there is no data or wrong type in the source then set error for the task
3. Validate data in a place(KEY\_JSON\_DATA) in the task - if the data is invalid json then set error for the task
4. Post data in a place(KEY\_JSON\_DATA) in the task as HTTP Body - if the response is unexpected then set error for the task

You can see all plugins just care their own jobs, and it just need the user to predefine interface between them. And the pipeline creates a new blank task for every new process.  the task will be stopped by pipeline at any plugins which consider the task is invalid for itself, then destroyed by pipeline or recovered by some plugins.

Now consider if the `json-validator.logs` needs to save all successful and failed number of validation, the simple but wrong method is to save them in fields of plugin struct. But here comes a problem, if user updates some configs like json schema, the numbers will be destroyed because the pipeline will reconstruct `json_validator.logs` into a new plugin instance . In this case, the life-cycle of total numbers should be the same with plugin(with a same name) not plugin instance. That's the main goal of the `PipelineContextDataBucket` we design. We can save all plugin-life-cycle data into a `PipelineContextDataBucket`. Moreover, we design the `PipelineStatistics` which provides a uniform API for exposing critical information in Pipeline or Plugin.

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
Plugin config needs to inherit `plugins.CommonConfig` which defines essential fields.  And it should hide intermediate products such as json schema object in `json_validator` .
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

### Demo Step by Step
Here's a demo to illustrate how to develop `json_validator` above step by step.

#### Step 1: Write Config
```golang
type jsonValidatorConfig struct {
	// inherit essential fields
	common.PluginCommonConfig
	// json Schema config from user
	Schema string `json:"schema"`
	// key of data source in task
	DataKey string `json:"data_key"`

	// schema object generated from Schema by gojsonschema.NewSchema
	// highly recommend to hide intermediate fields
	schemaObj *gojsonschema.Schema
}

// JSONValidatorConfigConstructor returns default config of JSONValidator
func JSONValidatorConfigConstructor() plugins.Config {
	return &jsonValidatorConfig{
		Schema:    "",
		DataKey:   "",
		schemaObj: nil,
	}
}

// Prepare validates config and does some preparation work for config
// after validating final config. This function is invoked after loading
// final config into jsonValidatorConfig
func (c *jsonValidatorConfig) Prepare(pipelineNames []string) error {
	// prepare common config firstly
	err := c.PluginCommonConfig.Prepare(pipelineNames)
	if err != nil {
		return err
	}

	// generate json schema
	loader := gojsonschema.NewBytesLoader([]byte(c.Schema))
	c.schemaObj, err = gojsonschema.NewSchema(loader)
	if err != nil {
		return fmt.Errorf("invalid schema: %v", err)
	}

	// It is recommended to remove all leading and trailing white space
	// which is not friendly to eyes of human.
	ts := strings.TrimSpace
	c.DataKey = ts(c.DataKey)

	// not allow empty key
	if len(c.DataKey) == 0 {
		return fmt.Errorf("invalid data key")
	}

	return nil
}
```

#### Step 2: Write Main Logic
```golang
type jsonValidator struct {
	// a essential field to save config
	conf *jsonValidatorConfig
}

// JSONValidatorConstructor is invoked to contruct a new instance.
func JSONValidatorConstructor(conf plugins.Config) (plugins.Plugin, error) {
	// Even gateway ensures type of entered config is corresponding Config
	// which is jsonValidatorConfig here, it's safer to use
	// checking  two values of type assertion to prevent panic.
	c, ok := conf.(*jsonValidatorConfig)
	if !ok {
		return nil, fmt.Errorf("config type want *jsonValidatorConfig got %T", conf)
	}

	return &jsonValidator{
		conf: c,
	}, nil
}

func (v *jsonValidator) Prepare(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (v *jsonValidator) validate(t task.Task) (error, task.TaskResultCode, task.Task) {
	// defensive programming
	if v.conf.schemaObj == nil {
		return fmt.Errorf("schema not found"), task.ResultInternalServerError, t
	}

	// read data from field of current task
	dataValue := t.Value(v.conf.DataKey)
	data, ok := dataValue.([]byte)
	if !ok {
		return fmt.Errorf("input %s got wrong value: %#v", v.conf.DataKey, dataValue),
			task.ResultMissingInput, t
	}

	// validating
	res, err := v.conf.schemaObj.Validate(gojsonschema.NewBytesLoader(data))
	if err != nil {
		return err, task.ResultBadInput, t
	}

	if !res.Valid() {
		var errs []string
		for i, err := range res.Errors() {
			errs = append(errs, fmt.Sprintf("%d: %s", i+1, err))
		}

		return fmt.Errorf(strings.Join(errs, ", ")), task.ResultBadInput, t
	}

	return nil, t.ResultCode(), t
}

func (v *jsonValidator) Run(ctx pipelines.PipelineContext, t task.Task) (task.Task, error) {
	err, resultCode, t := v.validate(t)
	if err != nil {
		t.SetError(err, resultCode)
	}
	// If it occurs some errors which jsonValidator can't handle and it needs
	// to reconstruct, the right way is to write `return t, err`.
	return t, nil
}

func (v *jsonValidator) Name() string {
	// PluginName is provided by CommonConfig
	return v.conf.PluginName()
}

func (v *jsonValidator) Close() {
	// meaningless operations but for demonstration effect
	release := func(interface{}) {}
	release(v.conf.schemaObj)
}
```

#### Step 3: Add Statistics
Bad:
```golang
type jsonValidator struct {
	conf *jsonValidatorConfig

	// these variables below are plugin-instance-cycle-life
	successfulCount uint64 // +
	failedCount     uint64 // +
}

func JSONValidatorConstructor(conf Config) (Plugin, error) {
	c, ok := conf.(*jsonValidatorConfig)
	if !ok {
		return nil, fmt.Errorf("config type want *jsonValidatorConfig got %T", conf)
	}

	return &jsonValidator{
		conf: c,
		// add redundant setting zero default value for demonstration effect
		successfulCount: 0, // +
		failedCount:     0, // +
	}, nil
}

func (v *jsonValidator) Prepare(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (v *jsonValidator) validate(t task.Task) (error, task.TaskResultCode, task.Task) {
	if v.conf.schemaObj == nil {
		return fmt.Errorf("schema not found"), task.ResultInternalServerError, t
	}

	dataValue := t.Value(v.conf.DataKey)
	data, ok := dataValue.([]byte)
	if !ok {
		return fmt.Errorf("input %s got wrong value: %#v", v.conf.DataKey, dataValue),
			task.ResultMissingInput, t
	}

	res, err := v.conf.schemaObj.Validate(gojsonschema.NewBytesLoader(data))
	if err != nil {
		// use atomic to fix race condition of multiple instances
		atomic.AddUint64(&v.failedCount, 1) // +
		return err, task.ResultBadInput, t
	}

	if !res.Valid() {
		var errs []string
		for i, err := range res.Errors() {
			errs = append(errs, fmt.Sprintf("%d: %s", i+1, err))
		}

		atomic.AddUint64(&v.failedCount, 1) // +
		return fmt.Errorf(strings.Join(errs, ", ")), task.ResultBadInput, t
	}

	atomic.AddUint64(&v.successfulCount, 1) // +
	return nil, t.ResultCode(), t
}

func (v *jsonValidator) Run(ctx pipelines.PipelineContext, t task.Task) (task.Task, error) {
	err, resultCode, t := v.validate(t)
	if err != nil {
		t.SetError(err, resultCode)
	}

	return t, nil
}

func (v *jsonValidator) Name() string {
	return v.conf.PluginName()
}

func (v *jsonValidator) Close() {
	release := func(interface{}) {}
	release(v.conf.schemaObj)
	release(v.successfulCount) // +
	release(v.failedCount)     // +
}
```
The version above is bad because different versions of `json_validator` with same plugin name can't share the successful and failed count. That means after the old `json_validator.logs` `Close`d, the new `json_validator.logs` with same plugin name and maybe different schema can't inherit the existing count. This is a common situation fixed by PipelineContext.

Good:
```golang
type jsonValidator struct {
	conf *jsonValidatorConfig
	// instanceId is a field to identify the unique instance of jsonValidator
	instanceId string // +
}

func JSONValidatorConstructor(conf Config) (Plugin, error) {
	c, ok := conf.(*jsonValidatorConfig)
	if !ok {
		return nil, fmt.Errorf("config type want *jsonValidatorConfig got %T", conf)
	}

	v := &jsonValidator{
		conf: c,
	}

	// it's recommended to use memory address
	v.instanceId = fmt.Sprintf("%p", &v) // +
	return v, nil
}

func (v *jsonValidator) registerSuccessfulCount(ctx pipelines.PipelineContext) { // +
	added, err := ctx.Statistics().RegisterPluginIndicator(
		v.Name(), v.instanceId, "SUCCESSFUL_VALIDATION_COUNT",
		fmt.Sprintf("The count of total successful count of validation"),
		func(pluginName, indicatorName string) (interface{}, error) {
			count, err := v.getSuccessfulCount(ctx)
			if err != nil {
				return nil, err
			}
			return *count, err
		},
	)
	if err != nil {
		logger.Warnf("[BUG: register plugin %s indicator %s failed, "+
			"ignored to expose customized statistics indicator: %s]", v.Name(), "SUCCESSFUL_VALIDATION_COUNT", err)
		return
	}

	if added {
		ctx.Statistics().UnregisterPluginIndicatorAfterPluginDelete(
			v.Name(), v.instanceId, "SUCCESSFUL_VALIDATION_COUNT",
		)
		ctx.Statistics().UnregisterPluginIndicatorAfterPluginUpdate(
			v.Name(), v.instanceId, "SUCCESSFUL_VALIDATION_COUNT",
		)
	}
}

func (v *jsonValidator) registerFailedCount(ctx pipelines.PipelineContext) { // +
	added, err := ctx.Statistics().RegisterPluginIndicator(
		v.Name(), v.instanceId, "FAILED_VALIDATION_COUNT",
		fmt.Sprintf("The count of total failed count of validation"),
		func(pluginName, indicatorName string) (interface{}, error) {
			count, err := v.getFailedCount(ctx)
			if err != nil {
				return nil, err
			}
			return *count, err
		},
	)
	if err != nil {
		logger.Warnf("[BUG: register plugin %s indicator %s failed, "+
			"ignored to expose customized statistics indicator: %s]", v.Name(), "FAILED_VALIDATION_COUNT", err)
		return
	}

	if added {
		ctx.Statistics().UnregisterPluginIndicatorAfterPluginDelete(
			v.Name(), v.instanceId, "FAILED_VALIDATION_COUNT",
		)
		ctx.Statistics().UnregisterPluginIndicatorAfterPluginUpdate(
			v.Name(), v.instanceId, "FAILED_VALIDATION_COUNT",
		)
	}
}

func (v *jsonValidator) Prepare(ctx pipelines.PipelineContext) {
	v.registerSuccessfulCount(ctx) // +
	v.registerFailedCount(ctx)     // +
}

const ( // +
	jsonValidatorSuccessfulCountKey = "jsonValidatorSuccessfulCountKey"
	jsonValidatorFailedCountKey     = "jsonValidatorFailedCountKey"
)

func (v *jsonValidator) getSuccessfulCount(ctx pipelines.PipelineContext) (*uint64, error) { // +
	// bucket := ctx.DataBucket(v.Name(), v.instanceId) means the bucket is
	// plugin-instance-life-cycle, it will be cleaned by PipeContext automatically
	bucket := ctx.DataBucket(v.Name(), pipelines.DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE)
	count, err := bucket.QueryDataWithBindDefault(jsonValidatorSuccessfulCountKey,
		func() interface{} {
			var successfulCount uint64
			return &successfulCount
		})
	if err != nil {
		logger.Warnf("[BUG: query state data for pipeline %s failed, "+
			"ignored to count validation: %s]", ctx.PipelineName(), err)
		return nil, err
	}

	return count.(*uint64), nil
}

func (v *jsonValidator) getFailedCount(ctx pipelines.PipelineContext) (*uint64, error) { // +
	bucket := ctx.DataBucket(v.Name(), pipelines.DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE)
	count, err := bucket.QueryDataWithBindDefault(jsonValidatorFailedCountKey,
		func() interface{} {
			var failedCount uint64
			return &failedCount
		})
	if err != nil {
		logger.Warnf("[BUG: query state data for pipeline %s failed, "+
			"ignored to count validation: %s]", ctx.PipelineName(), err)
		return nil, err
	}

	return count.(*uint64), nil
}

// pass one more parameter ctx
func (v *jsonValidator) validate(ctx pipelines.PipelineContext, t task.Task) (error, task.TaskResultCode, task.Task) {
	if v.conf.schemaObj == nil {
		return fmt.Errorf("schema not found"), task.ResultInternalServerError, t
	}

	dataValue := t.Value(v.conf.DataKey)
	data, ok := dataValue.([]byte)
	if !ok {
		return fmt.Errorf("input %s got wrong value: %#v", v.conf.DataKey, dataValue),
			task.ResultMissingInput, t
	}

	res, err := v.conf.schemaObj.Validate(gojsonschema.NewBytesLoader(data))
	if err != nil {
		failedCount, err := v.getFailedCount(ctx) // +
		if err != nil {                           // +
			return nil, t.ResultCode(), t // +
		} // +
		atomic.AddUint64(failedCount, 1) // +
		return err, task.ResultBadInput, t
	}

	if !res.Valid() {
		var errs []string
		for i, err := range res.Errors() {
			errs = append(errs, fmt.Sprintf("%d: %s", i+1, err))
		}

		failedCount, err := v.getFailedCount(ctx) // +
		if err != nil {                           // +
			return nil, t.ResultCode(), t // +
		} // +
		atomic.AddUint64(failedCount, 1) // +
		return fmt.Errorf(strings.Join(errs, ", ")), task.ResultBadInput, t
	}

	successfulCount, err := v.getSuccessfulCount(ctx) // +
	if err != nil {                                   // +
		return nil, t.ResultCode(), t // +
	} // +
	atomic.AddUint64(successfulCount, 1) // +
	return nil, t.ResultCode(), t
}

func (v *jsonValidator) Run(ctx pipelines.PipelineContext, t task.Task) (task.Task, error) {
	// pass one more parameter ctx
	err, resultCode, t := v.validate(ctx, t)
	if err != nil {
		t.SetError(err, resultCode)
	}
	return t, nil
}

func (v *jsonValidator) Name() string {
	return v.conf.PluginName()
}

func (v *jsonValidator) Close() {
	release := func(interface{}) {}
	release(v.conf.schemaObj)
}
```

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
