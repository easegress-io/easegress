package api

import (
	"fmt"

	"github.com/kataras/iris"
)

func handlAPIError(ctx iris.Context, code int, err error) {
	ctx.StatusCode(code)
	ctx.WriteString(fmt.Sprintf(`{"code":%d,"message":%s}`, code, err.Error()))
}
