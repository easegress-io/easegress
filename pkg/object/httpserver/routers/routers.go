package routers

import (
	"fmt"

	"github.com/megaease/easegress/pkg/object/httpserver"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
)

type (
	Kind struct {
		// Name is the name of the router kind.
		Name string

		// Description is the description of the router.
		Description string

		// CreateInstance creates a new router instance of the kind.
		CreateInstance func(rules httpserver.Rules) Router
	}
	RouteContext struct {
		Path     string
		Request  *httpprot.Request
		Captures map[string]string

		Code  int
		Route Route
	}

	Router interface {
		Search(context *RouteContext)
	}

	Route interface {
		Rewrite(context *RouteContext)
	}
)

var kinds = map[string]*Kind{}

func Register(k *Kind) {
	name := k.Name
	if name == "" {
		panic(fmt.Errorf("%T: empty router name", k))
	}

	if k1 := kinds[k.Name]; k1 != nil {
		msgFmt := "%T and %T got same name: %s"
		panic(fmt.Errorf(msgFmt, k, k1, k.Name))
	}

	kinds[name] = k
}

// Create creates a router instance of kind.
func Create(kind string, rules httpserver.Rules) Router {
	k := kinds[kind]
	if k == nil {
		return nil
	}
	return k.CreateInstance(rules)
}

func NewContext(req *httpprot.Request) *RouteContext {
	path := req.Path()

	context := &RouteContext{
		Path:    path,
		Request: req,
	}

	return context
}
