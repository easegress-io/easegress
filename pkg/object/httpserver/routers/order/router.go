package order

import (
	"github.com/megaease/easegress/pkg/object/httpserver"
	"github.com/megaease/easegress/pkg/object/httpserver/routers"
)

type (
	orderRouter struct {
		rules httpserver.Rules
	}
)

// Kind is the kind of Proxy.
const Kind = "Order"

var kind = &routers.Kind{
	Name:        Kind,
	Description: "Order",

	CreateInstance: func(rules httpserver.Rules) routers.Router {
		return &orderRouter{}
	},
}

func init() {
	routers.Register(kind)
}

func (r *orderRouter) Search(context *routers.RouteContext) {
}
