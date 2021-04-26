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

// ClusterPanic panics because of the cluster-level fault.
func ClusterPanic(err error) {
	panic(clusterErr(err.Error()))
}

// HandleAPIError handles api error.
func HandleAPIError(ctx iris.Context, code int, err error) {
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
