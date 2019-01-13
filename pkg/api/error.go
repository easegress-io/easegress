package api

import (
	"encoding/json"

	"github.com/kataras/iris"
)

type (
	clusterErr error

	APIErr struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	}
)

func handleAPIError(ctx iris.Context, code int, err error) {
	ctx.StatusCode(code)
	buff, err := json.Marshal(APIErr{
		Code:    code,
		Message: err.Error(),
	})
	if err != nil {
		panic(err)
	}
	ctx.Write(buff)
}
