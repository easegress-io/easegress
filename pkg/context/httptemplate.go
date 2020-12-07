package context

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/util/texttemplate"
)

const (
	pluginHTTPReqPath       = "plugin.%s.req.path"
	pluginHTTPReqMethod     = "plugin.%s.req.method"
	pluginHTTPReqBody       = "plugin.%s.req.body"
	pluginHTTPReqScheme     = "plugin.%s.req.scheme"
	pluginHTTPReqProto      = "plugin.%s.req.proto"
	pluginHTTPReqhost       = "plugin.%s.req.host"
	pluginHTTPReqheader     = "plugin.%s.req.header.%s"
	pluginHTTPRspStatusCode = "plugin.%s.rsp.statuscode"
	pluginHTTPRspBody       = "plugin.%s.rsp.body"

	defaultMaxBodySize = 10240
)

// HTTPTemplate is the wrapper of template engine for EaseGateway
type HTTPTemplate struct {
	Engine        texttemplate.TemplateEngine
	metaTemplates []string
}

func newHTTPTemplate() *HTTPTemplate {
	metaTemplates := []string{
		"plugin.{}.req.method",
		"plugin.{}.req.body",
		"plugin.{}.req.scheme",
		"plugin.{}.req.proto",
		"plugin.{}.req.host",
		"plugin.{}.req.body.{gjson}",
		"plugin.{}.req.header.{}",
		"plugin.{}.rsp.statuscode",
		"plugin.{}.rsp.body.{gjson}",
	}

	engine, err := texttemplate.NewDefault(metaTemplates)
	// even something wrong, we have the dummy template for using
	if err != nil {
		logger.Errorf("init http template fail [%v]", err)
	}

	return &HTTPTemplate{
		Engine:        engine,
		metaTemplates: metaTemplates,
	}
}

func readBody(body io.Reader, maxBodySize int64) (*bytes.Buffer, error) {
	buff := bytes.NewBuffer(nil)
	written, err := io.CopyN(buff, body, defaultMaxBodySize+1)

	if err != nil && err != io.EOF {
		err = fmt.Errorf("read body failed: %v", err)
		return nil, err
	}

	if written > defaultMaxBodySize {
		err = fmt.Errorf("body exceed %dB", defaultMaxBodySize)
		return nil, err
	}

	return buff, nil
}

// SaveRequest transfors HTTPRequest related fields into template engine's dictionary
func (e *HTTPTemplate) SaveRequest(pluginName string, ctx HTTPContext) error {
	bodyBuff, err := readBody(ctx.Request().Body(), defaultMaxBodySize)

	if err != nil {
		logger.Errorf("save HTTP request  body failed err %v", err)
		return err
	}

	e.Engine.SetDict(fmt.Sprintf(pluginHTTPReqBody, pluginName), string(bodyBuff.Bytes()))

	// Set the HTTP request Header
	for k, v := range ctx.Request().Std().Header {
		if len(v) > 0 {
			if len(v) == 1 {
				e.Engine.SetDict(fmt.Sprintf(pluginHTTPReqheader, pluginName, k), v[0])
			} else {
				// one header field with multiple values, join them with "," according to
				// https://stackoverflow.com/q/3096888/1705845
				e.Engine.SetDict(fmt.Sprintf(pluginHTTPReqheader, pluginName, k), strings.Join(v, ","))
			}
		}
	}

	// Set other HTTP request fields into dictionary
	e.Engine.SetDict(fmt.Sprintf(pluginHTTPReqMethod, pluginName), ctx.Request().Method())
	e.Engine.SetDict(fmt.Sprintf(pluginHTTPReqhost, pluginName), ctx.Request().Host())
	e.Engine.SetDict(fmt.Sprintf(pluginHTTPReqPath, pluginName), ctx.Request().Path())
	e.Engine.SetDict(fmt.Sprintf(pluginHTTPReqProto, pluginName), ctx.Request().Proto())
	e.Engine.SetDict(fmt.Sprintf(pluginHTTPReqScheme, pluginName), ctx.Request().Scheme())

	// Reset back into Request's Body
	ctx.Request().SetBody(bodyBuff)

	return err
}

// SaveResponse transfors HTTPResonse related fields into template engine's dictionary
func (e *HTTPTemplate) SaveResponse(pluginName string, ctx HTTPContext) error {
	bodyBuff, err := readBody(ctx.Response().Body(), defaultMaxBodySize)

	if err != nil {
		logger.Errorf("save HTTP response body failed err %v", err)
		return err
	}

	// set body and status code
	e.Engine.SetDict(fmt.Sprintf(pluginHTTPRspBody, pluginName), string(bodyBuff.Bytes()))
	e.Engine.SetDict(fmt.Sprintf(pluginHTTPRspStatusCode, pluginName), strconv.Itoa(ctx.Response().StatusCode()))

	//reset back to the HTTP body
	ctx.Response().SetBody(bodyBuff)
	return err
}

// Render using enginer to render template
func (e *HTTPTemplate) Render(input string) (string, error) {
	return e.Engine.Render(input)
}
