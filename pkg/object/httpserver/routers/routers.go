package router

import (
	"fmt"
)

type (
	SearchContext struct{}

	Router interface {
		Name() string
		Search(context *SearchContext)
		Rewrite(context *SearchContext)
	}
)

var routers = map[string]Router{}

func Register(r Router) {
	name := r.Name()
	if name == "" {
		panic(fmt.Errorf("%T: empty router name", r))
	}

	if r1 := routers[name]; r1 != nil {
		msgFmt := "%T and %T got same name: %s"
		panic(fmt.Errorf(msgFmt, r, r1, name))
	}

	routers[name] = r
}
