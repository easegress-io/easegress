package routers

import (
	"net/http"
	"testing"

	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/stretchr/testify/assert"
)

func TestGetCaptures(t *testing.T) {
	assert := assert.New(t)

	stdr, _ := http.NewRequest(http.MethodGet, "http://www.megaease.com:8080", nil)
	req, _ := httpprot.NewRequest(stdr)

	ctx := NewContext(req)
	res := ctx.GetCaptures()
	assert.Equal(0, len(res))
	assert.NotNil(res)

	ctx = NewContext(req)
	ctx.RouteParams.Keys = []string{"a", "b", "c"}
	ctx.RouteParams.Values = []string{"1", "2", "3"}
	res = ctx.GetCaptures()
	assert.Equal(3, len(res))
	assert.NotNil(res)
	assert.Equal(map[string]string{
		"a": "1",
		"b": "2",
		"c": "3",
	}, res)

	res = ctx.GetCaptures()
	assert.Equal(3, len(res))
	assert.NotNil(res)
	assert.Equal(map[string]string{
		"a": "1",
		"b": "2",
		"c": "3",
	}, res)
}
