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
	pluginReqPath       = "plugin.%s.req.path"
	pluginReqMethod     = "plugin.%s.req.method"
	pluginReqBody       = "plugin.%s.req.body"
	pluginReqScheme     = "plugin.%s.req.scheme"
	pluginReqProto      = "plugin.%s.req.proto"
	pluginReqhost       = "plugin.%s.req.host"
	pluginReqheader     = "plugin.%s.req.header.%s"
	pluginRspStatusCode = "plugin.%s.rsp.statuscode"
	pluginRspBody       = "plugin.%s.rsp.body"

	defaultMaxBodySize = 10240
	defaultTagNum      = 4

	pluginNameTagIndex   = 1
	pluginReqRspTagIndex = 2
	pluginValueTagIndex  = 3
)

type (
	// HTTPTemplate is the wrapper of template engine for EaseGateway
	HTTPTemplate struct {
		Engine        texttemplate.TemplateEngine
		metaTemplates []string

		// store the plugin name and its
		// calling function lists
		pluginExecFuncs map[string]pluginDictFuncs

		// the dependency order array of plugins
		pluginsOrder []string
	}

	setDictFunc func(*HTTPTemplate, string, HTTPContext) error

	pluginDictFuncs struct {
		reqFuncs []setDictFunc
		rspFuncs []setDictFunc
	}

	// PluginBuff stores plugin's name and its YAML bytes
	PluginBuff struct {
		Name string
		Buff []byte
	}
)

var (
	metaTemplates = []string{
		"plugin.{}.req.method",
		"plugin.{}.req.body",
		"plugin.{}.req.scheme",
		"plugin.{}.req.path",
		"plugin.{}.req.proto",
		"plugin.{}.req.host",
		"plugin.{}.req.body.{gjson}",
		"plugin.{}.req.header.{}",
		"plugin.{}.rsp.statuscode",
		"plugin.{}.rsp.body.{gjson}",
	}

	tagsFuncMap = map[string]setDictFunc{
		"req.method":     saveReqMethod,
		"req.body":       saveReqBody,
		"req.path":       saveReqPath,
		"req.scheme":     saveReqScheme,
		"req.proto":      saveReqProto,
		"req.host":       saveReqHost,
		"req.header":     saveReqHeader,
		"rsp.statuscode": saveRspStatuscode,
		"rsp.body":       saveRspBody,
	}
)

// NewHTTPTemplate returns a default HTTPTemplate
func NewHTTPTemplate(pluginBuffs []PluginBuff) (*HTTPTemplate, error) {
	engine, err := texttemplate.NewDefault(metaTemplates)
	if err != nil {
		logger.Errorf("init http template fail [%v]", err)
		return nil, err
	}

	e := HTTPTemplate{
		Engine:          engine,
		metaTemplates:   metaTemplates,
		pluginExecFuncs: map[string]pluginDictFuncs{},
	}

	pluginFuncTags := map[string][]string{}
	// validates the plugin's YAML spec for dependency checking
	// and template format,e.g., if plugin1 has a template said '[[plugin.plugin2.rsp.data]],
	// but it appears before plugin2, then it's an invalidate dependency cause we can't get
	// the rsp form plugin2 in the execution period of plugin1. At last it will build up
	// executing function arraies for every plugins.
	for _, pluginBuff := range pluginBuffs {
		e.pluginsOrder = append(e.pluginsOrder, pluginBuff.Name)
		templatesMap := e.Engine.ExtractRawTemplateRuleMap(string(pluginBuff.Buff))
		if len(templatesMap) == 0 {
			continue
		}
		dependPlugins := []string{}
		for template, renderMeta := range templatesMap {
			// no matched and rendered meta template
			if len(renderMeta) == 0 {
				err = fmt.Errorf("plugin %s template [[%s]] check failed, unregonized", pluginBuff.Name, template)
				break
			}

			tags := strings.Split(renderMeta, texttemplate.DefaultSepertor)
			if len(tags) < defaultTagNum {
				err = fmt.Errorf("plugin %s template [[%s]] check failed,its render metaTemplate [[%s]] is invalid",
					pluginBuff.Name, template, renderMeta)
				break
			} else {
				dependPluginName := tags[pluginNameTagIndex]
				dependPlugins = append(dependPlugins, dependPluginName)
				funcTag := tags[pluginReqRspTagIndex] + texttemplate.DefaultSepertor +
					tags[pluginValueTagIndex]

				funcTags := pluginFuncTags[dependPluginName]
				found := false
				for _, v := range funcTags {
					if funcTag == v {
						found = true
						break
					}
				}
				if !found {
					pluginFuncTags[dependPluginName] = append(funcTags, funcTag)
				}
			}
		}
		if err != nil {
			return nil, err
		}
		// get its all rely plugins and make sure these targets have already show
		// up, and couldn't rely on itself.
		if err = e.validatePluginDependency(pluginBuff.Name, dependPlugins); err != nil {
			return nil, err
		}
	}

	e.storePluginExecFuncs(pluginFuncTags)

	return &e, nil
}

// NewHTTPTemplateDummy return a empty implement version of HTTP template
func NewHTTPTemplateDummy() *HTTPTemplate {
	return &HTTPTemplate{
		Engine:          texttemplate.NewDummyTemplate(),
		pluginExecFuncs: map[string]pluginDictFuncs{},
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
	var (
		execFuncs pluginDictFuncs
		ok        bool
	)
	if execFuncs, ok = e.pluginExecFuncs[pluginName]; !ok {
		return nil
	}

	if len(execFuncs.reqFuncs) == 0 {
		return nil
	}

	for _, f := range execFuncs.reqFuncs {
		if err := f(e, pluginName, ctx); err != nil {
			logger.Errorf("plugin %s execute HTTP template req func %v failed err %v",
				pluginName, f, err)
			return err
		}
	}

	return nil
}

// SaveResponse transfors HTTPResonse related fields into template engine's dictionary
func (e *HTTPTemplate) SaveResponse(pluginName string, ctx HTTPContext) error {
	var (
		execFuncs pluginDictFuncs
		ok        bool
	)
	if execFuncs, ok = e.pluginExecFuncs[pluginName]; !ok {
		return nil
	}

	if len(execFuncs.rspFuncs) == 0 {
		return nil
	}

	for _, f := range execFuncs.rspFuncs {
		if err := f(e, pluginName, ctx); err != nil {
			logger.Errorf("plugin %s execute HTTP template rsp func %v failed err %v",
				pluginName, f, err)
			return err
		}
	}

	return nil
}

// Render using enginer to render template
func (e *HTTPTemplate) Render(input string) (string, error) {
	return e.Engine.Render(input)
}

func (e *HTTPTemplate) validatePluginDependency(pluginName string, dependPlugins []string) error {
	var err error
	for _, name := range dependPlugins {
		if pluginName == name {
			err = fmt.Errorf("plugin %s template check failed, rely on itself", pluginName)
			break
		}
		found := false
		for _, existPlugin := range e.pluginsOrder {
			if existPlugin == name {
				found = true
				break
			}
		}

		if !found {
			err = fmt.Errorf("plugin %s template check failed, rely on a havn't executed plugin %s ",
				pluginName, name)
			break
		}
	}
	return err
}

func (e *HTTPTemplate) storePluginExecFuncs(pluginFuncTags map[string][]string) {
	for pluginName, funcTags := range pluginFuncTags {
		var (
			execFuncs pluginDictFuncs
			ok        bool
		)
		if execFuncs, ok = e.pluginExecFuncs[pluginName]; !ok {
			e.pluginExecFuncs[pluginName] = pluginDictFuncs{}
		}

		for _, funcTag := range funcTags {
			if f, ok := tagsFuncMap[funcTag]; ok {
				if strings.Contains(funcTag, "req") {
					execFuncs.reqFuncs = append(execFuncs.reqFuncs, f)
				} else {
					execFuncs.rspFuncs = append(execFuncs.rspFuncs, f)
				}
			}
		}
		e.pluginExecFuncs[pluginName] = execFuncs
	}
}

func saveRspStatuscode(e *HTTPTemplate, pluginName string, ctx HTTPContext) error {
	return e.Engine.SetDict(fmt.Sprintf(pluginRspStatusCode, pluginName), strconv.Itoa(ctx.Response().StatusCode()))
}

func saveReqHost(e *HTTPTemplate, pluginName string, ctx HTTPContext) error {
	return e.Engine.SetDict(fmt.Sprintf(pluginReqhost, pluginName), ctx.Request().Host())
}

func saveReqPath(e *HTTPTemplate, pluginName string, ctx HTTPContext) error {
	return e.Engine.SetDict(fmt.Sprintf(pluginReqPath, pluginName), ctx.Request().Path())
}
func saveReqProto(e *HTTPTemplate, pluginName string, ctx HTTPContext) error {
	return e.Engine.SetDict(fmt.Sprintf(pluginReqProto, pluginName), ctx.Request().Proto())
}

func saveReqScheme(e *HTTPTemplate, pluginName string, ctx HTTPContext) error {
	return e.Engine.SetDict(fmt.Sprintf(pluginReqScheme, pluginName), ctx.Request().Scheme())
}

func saveReqMethod(e *HTTPTemplate, pluginName string, ctx HTTPContext) error {
	return e.Engine.SetDict(fmt.Sprintf(pluginReqMethod, pluginName), ctx.Request().Method())
}

func saveReqBody(e *HTTPTemplate, pluginName string, ctx HTTPContext) error {
	bodyBuff, err := readBody(ctx.Request().Body(), defaultMaxBodySize)

	if err != nil {
		logger.Errorf("httptemplate save HTTP request  body failed err %v", err)
		return err
	}
	e.Engine.SetDict(fmt.Sprintf(pluginReqBody, pluginName), string(bodyBuff.Bytes()))
	// Reset back into Request's Body
	ctx.Request().SetBody(bodyBuff)

	return nil
}

func saveReqHeader(e *HTTPTemplate, pluginName string, ctx HTTPContext) error {
	// Set the HTTP request Header
	for k, v := range ctx.Request().Std().Header {
		if len(v) > 0 {
			if len(v) == 1 {
				e.Engine.SetDict(fmt.Sprintf(pluginReqheader, pluginName, k), v[0])
			} else {
				// one header field with multiple values, join them with "," according to
				// https://stackoverflow.com/q/3096888/1705845
				e.Engine.SetDict(fmt.Sprintf(pluginReqheader, pluginName, k), strings.Join(v, ","))
			}
		}
	}
	return nil
}

func saveRspBody(e *HTTPTemplate, pluginName string, ctx HTTPContext) error {
	bodyBuff, err := readBody(ctx.Response().Body(), defaultMaxBodySize)

	if err != nil {
		logger.Errorf("httptemplate save HTTP response body failed err %v", err)
		return err
	}
	e.Engine.SetDict(fmt.Sprintf(pluginRspBody, pluginName), string(bodyBuff.Bytes()))
	ctx.Response().SetBody(bodyBuff)
	return nil
}
