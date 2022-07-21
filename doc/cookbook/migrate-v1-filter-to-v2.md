# Migrate v1.x Filter To v2.0

Easegress v2.0 introduces exciting new features like protocol-independent
pipeline, multiple requests/responses support, etc. But this also makes
it incompatible with v1.x. We need to do some code modifications to bring
filters designed for v1.x to v2.0.

This document is a guide on how to do the migration, we will use the `Mock`
filter as an example.

## 1. Define a kind for the filter

```go
var kind = &filters.Kind{
	Name:        Kind,
	Description: "Mock mocks the response.",
	Results:     []string{resultMocked},
	DefaultSpec: func() filters.Spec {
		return &Spec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &Mock{spec: spec.(*Spec)}
	},
}
```

Note the value of the `Description` field is from the existing `Description`
function:

```go
func (m *Mock) Description() string {
	return "Mock mocks the response."
}
```

`Results` is from the existing `Results` function:

```go
var results = []string{resultMocked}

func (m *Mock) Results() []string {
	return results
}
```

The body `DefaultSpec` is just the same as the existing `DefaultSpec` function.

For `CreateInstance`, in most cases, we only need to return a new instance
of the filter, but please remember to set the `spec` field before returning it.

## 2. Remove legacy functions

The legacy functions, `Description`, `Results` and `DefaultSpec` function are
now useless, remove them.

## 3. Update Filter and its Spec definition

First, add `BaseSpec` to the `Spec` definition:

```go
Spec struct {
	filters.BaseSpec `yaml:",inline"`    // add this line
	Rules []*Rule `yaml:"rules"`
}
```

Then, remove the `filterSpec` field from the filter definition:

```go
type Mock struct {
	filterSpec *httppipeline.FilterSpec  // remove this line
	spec       *Spec
}
```

## 4. Add new function `Name`

```go
// Name returns the name of the Mock filter instance.
func (m *Mock) Name() string {
	return m.spec.Name()
}
```

## 5. Update the `Kind` function

In v1.x, The function returns the kind name, while in v2.x, it should return
the `kind` object we defined in step 1.

```go
// Kind returns the kind of Mock.
func (m *Mock) Kind() *filters.Kind {
	return kind
}
```

## 6. Update the `Init` and `Inherit` function

Change the prototype of these two functions to:

```go
func (m *Mock) Init()
```

and

```go
func (m *Mock) Inherit(previousGeneration filters.Filter)
```

The spec of the filter has already been assigned in step 1, so related code
can be removed from the two functions. And if you need to access the spec
from the functions, please use `m.spec` directly.

Note, in Easegress v2, the `previousGeneration` should NOT be closed in
`Inherit` any more, that's `previousGeneration.Close()` should be removed
from `Inherit`.

## 7. Update the `Handle` function

First change the prototype to below, note the type of the `ctx` parameter
has changed from `context.HTTPContext` to `*context.Context`:

```go
func (m *Mock) Handle(ctx *context.Context) string
```

Then, because Easegress is not using the chain of responsibility pattern to
call filters any more, we need to change the `return` statements from:

```go
return ctx.CallNextHandler(result)
```

to

```go
return result
```

## 8. Remove `httppipeline` From Imported Packages

```go
import (
	// ...
	"github.com/megaease/easegress/pkg/object/httppipeline"  // remove this line
	// ...
)

```

## 9. Update Other Code

`ctx.Request()` need to be updated to `ctx.GetInputRequest()`,
`ctx.GetOutputRequest()`, `ctx.SetInputRequest()` or `ctx.SetOutputRequest()`,
same for `ctx.Response()`. And please make a type assertion if you need a
protocol specific request/response, like
`ctx.GetInputRequest().(*httpprot.Request) if an HTTP request is desired.

The above is all the general steps to migrate a filter from v1.x to v2.x,
and you may need more works that are specific to your implementation to
complete the migration. We think most of the works would be trivial, but
please feel free to contact us if you need help.