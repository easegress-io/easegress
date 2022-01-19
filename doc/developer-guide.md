# Developer Guide

- [Developer Guide](#developer-guide)
	- [Architecture](#architecture)
	- [Project Layout](#project-layout)
	- [Building and testing](#building-and-testing)
- [Extending Easegress](#extending-easegress)
	- [Developing an Object](#developing-an-object)
		- [Main Business Logic](#main-business-logic)
		- [Register Object to Supervisor](#register-object-to-supervisor)
	- [Developing a Filter](#developing-a-filter)
		- [Main Business Logic](#main-business-logic-1)
		- [Register Filter to Pipeline](#register-filter-to-pipeline)
		- [JumpIf Mechanism in Pipeline](#jumpif-mechanism-in-pipeline)

## Architecture

The following diagram describes the high-level architecture of Easegress. The Cluster component is responsible for synchronizing data among different nodes or after restarts. The Supervisor manages the lifecycle of all kinds of objects.

![architecture](./imgs/architecture.png)

Easegress has four different *kind* of objects:
1. **System Controllers** operate essential system-level activities. Every instance of Easegress launches exactly one instance of each kind of System Controllers after the very start.
2. **Business Controllers** can be created and deleted using admin operations. Business Controllers operate mainly on other tasks than handling directly the traffic.
3. **Traffic Gate** receives traffic of different protocols, and dispatches them to pipelines.
4. **Pipeline** is a filter chain that handles traffic from the traffic gate.


## Project Layout

Easegress code structure follows the [go project layout standard](https://github.com/golang-standards/project-layout), and the important directories are described below:

```
.
├── bin					// executable binary
├── cmd					// command source code
├── doc					// documents
├── pkg					// importable golang packages
│   ├── api				// restful api layer
│   ├── cluster				// cluster component
│   ├── common				// some common utilies
│   ├── context				// context for traffic gate and pipeline
│   ├── env				// preparation for running environment
│   ├── filter				// filters
│   ├── graceupdate			// graceful update
│   ├── logger				// logger utilities
│   ├── object				// controllers
│   ├── option				// startup arguments utilities
│   ├── pidfile				// handle file to record pid
│   ├── profile				// dedicated pprof
│   ├── protocol			// decoupling for protocol
│   ├── registry			// registry for all dynamic registering component
│   ├── storage				// distributed storage wrapper
│   ├── supervisor			// the supervisor to manage controllers
│   ├── tracing				// distributed tracing
│   ├── util				// all kinds of utilities
│   ├── v				// validation tool
│   └── version				// release version
├── test				// scripts of integration testing
```

## Building and testing

If you are new to Easegress, please go through the  [Getting Started](../README.md#getting-started) tutorial,  if you have not already. Also try out different [Cookbook](./README.md) tutorials.

To build `easegress-server` and `egctl`, run `make` and they will appear to `bin` directory.

After modifying the code, run `make fmt` to format the code and `make test` to run unit tests.

# Extending Easegress

Let's suppose that you have a requirement or a feature enhancement in your mind. The most common way to extend Easegress is to develop a new Object or Filter.

## Developing an Object

The first thing is to choose which one of the four Object types is the most suitable for a given feature. In most cases, creating a new Business Controller is the best choice to extend the ability of Easegress at the object level. So let's develop a lightweight business controller to show the details. For example, let's say you want to develop a controller called `StatusInLocalController` that dumps the status of all objects to a local file. Here's the config of the controller:

```yaml
kind: StatusInLocalController
name: statusInLocal
path: ./running_status.yaml
```

### Main Business Logic

Put the controller package in `pkg/object/statusinlocalcontroller` and implement the main logic in `pkg/object/statusinlocalcontroller/statusinlocalcontroller.go`:

The supervisor has all references of running objects, so you need invoke supervisor to get status of running objects, and the main business code would be:

```go
type (
	// StatusInLocalController posts status of all objects in a local file.
	StatusInLocalController struct {
		superSpec *supervisor.Spec
		spec      *Spec

		done chan struct{}
	}

	// Spec describes StatusInLocalController.
	Spec struct {
		Path string `yaml:"path" jsonschema:"required"`
	}

	// Entry is the structure of the status file
	Entry struct {
		Statuses     map[string]interface{}
		UnixTimestamp int64
	}
)

func (c *StatusInLocalController) syncStatus() {
	// Step1: Use entry to record status of all running objects.
	entry := &Entry{
		UnixTimestamp: time.Now().Unix(),
		Statuses:      make(map[string]interface{}),
	}

	walkFn := func(entity *supervisor.ObjectEntity) bool {
		defer func() {
			if err := recover(); err != nil {
				logger.Errorf("recover from syncStatus, err: %v, stack trace:\n%s\n",
					err, debug.Stack())
			}
		}()

		name := entity.Spec().Name()
		entry.Statuses[name] = entity.Instance().Status()

		return true
	}

	c.superSpec.Super().WalkControllers(walkFn)

	// Step2: Write the status to the local file.
	buff, err := yaml.Marshal(entry)
	if err != nil {
		logger.Errorf("BUG: marshal %#v to yaml failed: %v",
			entry, err)
		return
	}

	os.WriteFile(c.spec.Path, buff, 0644)
}
```

### Register Object to Supervisor

All objects must satisfy the interface `Object` in [`pkg/object/supervisor/registry.go`](https://github.com/megaease/easegress/blob/master/pkg/supervisor/registry.go).

```go
package statusinlocalcontroller

import (
	"os"
	"runtime/debug"
	"time"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/supervisor"

	"gopkg.in/yaml.v2"
)

const (
	// Kind is the kind of StatusInLocalController.
	Kind = "StatusInLocalController"
)

type (
	// StatusInLocalController posts status of all objects in a local file.
	StatusInLocalController struct {
		superSpec *supervisor.Spec
		spec      *Spec

		done chan struct{}
	}

	// Spec describes StatusInLocalController.
	Spec struct {
		Path string `yaml:"path" jsonschema:"required"`
	}

	// Entry is the structure of the status file.
	Entry struct {
		Statuses      map[string]interface{}
		UnixTimestamp int64
	}
)

// init registers itself to supervisor registry.
func init() {
	supervisor.Register(&StatusInLocalController{})
}

func (c *StatusInLocalController) syncStatus() {
	// Step1: Use entry to record status of all running objects.
	entry := &Entry{
		UnixTimestamp: time.Now().Unix(),
		Statuses:      make(map[string]interface{}),
	}

	walkFn := func(entity *supervisor.ObjectEntity) bool {
		defer func() {
			if err := recover(); err != nil {
				logger.Errorf("recover from syncStatus, err: %v, stack trace:\n%s\n",
					err, debug.Stack())
			}
		}()

		name := entity.Spec().Name()
		entry.Statuses[name] = entity.Instance().Status()

		return true
	}

	c.superSpec.Super().WalkControllers(walkFn)

	// Step2: Write the status to the local file.
	buff, err := yaml.Marshal(entry)
	if err != nil {
		logger.Errorf("BUG: marshal %#v to yaml failed: %v",
			entry, err)
		return
	}

	os.WriteFile(c.spec.Path, buff, 0644)
}

// Category returns the category of StatusInLocalController.
func (c *StatusInLocalController) Category() supervisor.ObjectCategory {
	return supervisor.CategoryBusinessController
}

// Kind return the kind of StatusInLocalController.
func (c *StatusInLocalController) Kind() string { return Kind }

// DefaultSpec returns the default spec of StatusInLocalController.
func (c *StatusInLocalController) DefaultSpec() interface{} { return &Spec{} }

// Init initializes StatusInLocalController.
func (c *StatusInLocalController) Init(superSpec *supervisor.Spec) {
	c.superSpec, c.spec = superSpec, superSpec.ObjectSpec().(*Spec)
	c.reload()
}

// Inherit inherits previous generation of StatusInLocalController.
func (c *StatusInLocalController) Inherit(spec *supervisor.Spec,
	previousGeneration supervisor.Object) {

	previousGeneration.Close()
	c.Init(spec)
}

func (c *StatusInLocalController) reload() {
	c.done = make(chan struct{})

	go c.run()
}

func (c *StatusInLocalController) run() {
	for {
		select {
		case <-time.After(5 * time.Second):
			c.syncStatus()
		case <-c.done:
			return
		}
	}
}

// Status returns the status of StatusInLocalController.
func (c *StatusInLocalController) Status() *supervisor.Status {
	return &supervisor.Status{
		ObjectStatus: struct{}{},
	}
}

// Close closes StatusInLocalController.
func (c *StatusInLocalController) Close() {
	close(c.done)
}
```

In the end, we have to import `StatusInLocalController` in `pkg/registry/registry.go`.
```go
import (
// Filters
// ...

// Objects
// ...
	_ "github.com/megaease/easegress/pkg/object/statusinlocalcontroller"
)

```

## Developing a Filter

In most scenarios of handling traffic, creating a new Filter is the right choice, since its scheduling is covered by the flexible Pipeline. Filters are executed in a Pipeline sequentially, each one being responsible for one step of the traffic handling. For example, let's develop a filter to count the number of requests which have the specified header. Let's name the kind of filter `headerCounter`, so the config of the filter in pipeline spec would be:

```yaml
filters:
- kind: HeaderCounter
  name: headerCounter
  headers: ['Cookie', 'Authorization']
```

### Main Business Logic

Put the filter package in `pkg/filter/headercounter`. You could implement the main logic counting the header in `pkg/filter/headercounter/headercounter.go`:

```go

type (
	HeaderCounter struct {
		pipeSpec *httppipeline.FilterSpec	// The filter spec in pipeline level, which has two more fiels: kind and name.
		spec     *Spec						// The filter spec in its own level.

		// The read and write for count must be locked, because the Handle is called concurrently.
		countMutex sync.Mutex
		count      map[string]int64
	}

	Spec struct {
		Headers []string `yaml:"headers"`
	}
)

func (m *HeaderCounter) Handle(ctx context.HTTPContext) (result string) {
	for _, key := range m.spec.Headers {
		value := ctx.Request().Header().Get(key)
		if value != "" {
			m.countMutex.Lock()
			m.count[key]++
			m.countMutex.Unlock()
		}
	}

	// NOTE: The filter must call the next handler to satisfy the Chain of Responsibility Pattern.
	return ctx.CallNextHandler("")
}

```

### Register Filter to Pipeline

Our core logic is very simple, now let's add some non-business code to make our new filter conform with the requirement of the Pipeline framework. All filters must satisfy the interface `Filter` in [`pkg/object/httppipeline/registry.go`](https://github.com/megaease/easegress/blob/master/pkg/object/httppipeline/registry.go).

All of the methods with their names and comments are clean, the only one we need to emphasize is `Inherit`. It is called when the pipeline is updated, without modifying the filter's identity (*name* and *kind*). In practice, this happens when the underlying machine reboots and restarts Easegress. It's the filter's own responsibility to do hot-update in `Inherit` such as transferring meaningful consecutive data.

```go
// init registers itself to pipeline registry.
func init() { httppipeline.Register(&HeaderCounter{}) }

// Kind returns the kind of HeaderCounter.
func (hc *HeaderCounter) Kind() string { return "HeaderCounter" }

// DefaultSpec returns default spec of HeaderCounter.
func (hc *HeaderCounter) DefaultSpec() interface{} { return &Spec{} }

// Description returns the description of HeaderCounter.
func (hc *HeaderCounter) Description() string {
	return "HeaderCounter counts the number of requests which contain the specified header."
}

// Results returns the results of HeaderCounter.
func (hc *HeaderCounter) Results() []string { return nil }

// Init initializes HeaderCounter.
func (hc *HeaderCounter) Init(pipeSpec *httppipeline.FilterSpec) {
	hc.pipeSpec, hc.spec = pipeSpec, pipeSpec.FilterSpec().(*Spec)
	hc.reload()
}

// Inherit inherits previous generation of HeaderCounter.
func (hc *HeaderCounter) Inherit(pipeSpec *httppipeline.FilterSpec,
	previousGeneration httppipeline.Filter) {

	previousGeneration.Close()
	hc.Init(pipeSpec)
}

func (m *HeaderCounter) reload() {
	m.count = make(map[string]int64)
}

// Status returns status.
func (m *HeaderCounter) Status() interface{} {
	m.countMutex.Lock()
	defer m.countMutex.Unlock()

	return m.count
}

// Close closes HeaderCounter.
func (m *HeaderCounter) Close() {}
```

Then we need to add the import line in the `pkg/registry/registry.go`:

```go
import (
	_ "github.com/megaease/easegress/pkg/filter/headercounter
)
```

### JumpIf Mechanism in Pipeline

The [Getting Started](../README.md#getting-started) part of the README uses briefly the `jumpIf` mechanism of the `HTTPPipeline`. Let's describe the concept of `jumpIf` using the example below:

```yaml
name: pipeline-demo
kind: HTTPPipeline
flow:
- filter: validator
  jumpIf: { invalid: END }
- filter: requestAdaptor
- filter: proxy
```

That `jumpIf` means the request will jump into the end of the `HTTPPipeline` without going through `requestAdaptor` and `proxy` if the `validator` returns the result `invalid`. On the other hand, when `validator` returns empty string `""` the request proceeds to next filter as usual.

So the purpose of the method `Results` in `Filter` interface is to register all possible results of the filter. In the example of `HeaderCounter`, the empty results mean `Handle` only returns the empty result. In order to use `jumpIf` to skip proceeding filters, `Results` function has to define output code for invalid execution. So if we want to prevent requests which haven't any counting headers from going forward to next filters, we could change it to:

```go
const resultInvalidHeader = "invalidHeader"

// Results returns the results of HeaderCounter.
func (hc *HeaderCounter) Results() []string {
	return []string{resultInvalidHeader} // New code
}

// Handle counts the header of the requests.
func (m *HeaderCounter) Handle(ctx context.HTTPContext) (result string) {
	counted := false // New code
	for _, key := range m.spec.Headers {
		value := ctx.Request().Header().Get(key)
		if value != "" {
			m.countMutex.Lock()
			counted = true // New code
			m.count[key]++
			m.countMutex.Unlock()
		}
	}

	if !counted { // New code
		// No headers counted; let's skip the rest of the pipeline
		return resultInvalidHeader // New code

	} // New code

	return ""
}
```
