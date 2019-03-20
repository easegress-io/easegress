package api

import (
	"github.com/kataras/iris"

	yaml "gopkg.in/yaml.v2"
)

type (
	clusterErr string

	// APIErr is the standard return of error.
	APIErr struct {
		Code    int    `yaml:"code"`
		Message string `yaml:"message"`
	}
)

func (ce clusterErr) Error() string {
	return string(ce)
}

func clusterPanic(err error) {
	panic(clusterErr(err.Error()))
}

func handleAPIError(ctx iris.Context, code int, err error) {
	ctx.StatusCode(code)
	buff, err := yaml.Marshal(APIErr{
		Code:    code,
		Message: err.Error(),
	})
	if err != nil {
		panic(err)
	}
	ctx.Write(buff)
}
