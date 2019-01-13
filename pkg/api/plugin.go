package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/megaease/easegateway/pkg/common"
	"github.com/megaease/easegateway/pkg/plugins"
	"github.com/megaease/easegateway/pkg/store"

	"github.com/kataras/iris"
)

func (s *APIServer) setupPluginAPIs() {
	pluginAPIs := []*apiEntry{
		{
			Path:    "/plugins",
			Method:  "POST",
			Handler: s.createPlugin,
		},
		{
			Path:    "/plugins/{name:string}",
			Method:  "PUT",
			Handler: s.updatePlugin,
		},
		{
			Path:    "/plugins/{name:string}",
			Method:  "GET",
			Handler: s.getPlugin,
		},
		{
			Path:    "/plugins/{name:string}",
			Method:  "DELETE",
			Handler: s.deletePlugin,
		},
		{
			Path:    "/plugins",
			Method:  "GET",
			Handler: s.listPlugins,
		},
		{
			Path:    "/plugin-types",
			Method:  "GET",
			Handler: s.listPluginTypes,
		},
	}

	s.apis = append(s.apis, pluginAPIs...)
}

func (s *APIServer) readPluginSpec(ctx iris.Context) (*store.PluginSpec, error) {
	body, err := ioutil.ReadAll(ctx.Request().Body)
	if err != nil {
		return nil, fmt.Errorf("read body failed: %v", err)
	}

	return store.NewPluginSpec(string(body))
}

func (s *APIServer) createPlugin(ctx iris.Context) {
	spec, err := s.readPluginSpec(ctx)
	if err != nil {
		handleAPIError(ctx, iris.StatusBadRequest, err)
		return
	}

	if err := common.ValidateName(spec.Name); err != nil {
		handleAPIError(ctx, iris.StatusBadRequest, err)
		return
	}

	s.Lock()
	defer s.Unlock()

	existedSpec := s._getPlugin(spec.Name)
	if existedSpec != nil {
		handleAPIError(ctx, iris.StatusConflict, fmt.Errorf("conflict name"))
		return
	}

	err = spec.Bootstrap(s._pipelineNames())
	if err != nil {
		handleAPIError(ctx, iris.StatusBadRequest, err)
		return
	}

	s._putPlugin(spec)

	ctx.StatusCode(iris.StatusCreated)
	location := fmt.Sprintf("%s/%s", ctx.Path(), spec.Name)
	ctx.Header("Location", location)
}

func (s *APIServer) deletePlugin(ctx iris.Context) {
	name := ctx.Params().Get("name")

	s.Lock()
	defer s.Unlock()

	spec := s._getPlugin(name)
	if spec == nil {
		handleAPIError(ctx, iris.StatusNotFound, fmt.Errorf("not found"))
		return
	}

	pipelines := s._pluginUsedByPipelines(name)
	if len(pipelines) != 0 {
		handleAPIError(ctx, http.StatusBadRequest,
			fmt.Errorf("plugin %s is occupied by pipelines %q",
				name, pipelines))
		return
	}

	s._deletePlugin(name)
}

func (s *APIServer) getPlugin(ctx iris.Context) {
	name := ctx.Params().Get("name")

	// No need to lock.

	spec := s._getPlugin(name)
	if spec == nil {
		handleAPIError(ctx, iris.StatusNotFound, fmt.Errorf("not found"))
		return
	}

	buff, err := json.Marshal(spec)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", spec, err))
	}

	ctx.Write(buff)
}

func (s *APIServer) updatePlugin(ctx iris.Context) {
	name := ctx.Params().Get("name")

	spec, err := s.readPluginSpec(ctx)
	if err != nil {
		handleAPIError(ctx, iris.StatusBadRequest, err)
		return
	}

	if name != spec.Name {
		handleAPIError(ctx, iris.StatusBadRequest,
			fmt.Errorf("spec got different name with url parameter"))
		return
	}

	s.Lock()
	defer s.Unlock()

	oldSpec := s._getPlugin(name)
	if oldSpec == nil {
		handleAPIError(ctx, iris.StatusNotFound, fmt.Errorf("not found"))
		return
	}

	if oldSpec.Type != spec.Type {
		handleAPIError(ctx, iris.StatusBadRequest,
			fmt.Errorf("type of plugin %s is %s, changed to %s",
				spec.Name, oldSpec.Type, spec.Type))
	}

	err = spec.Bootstrap(s._pipelineNames())
	if err != nil {
		handleAPIError(ctx, iris.StatusBadRequest, err)
		return
	}

	s._putPlugin(spec)
}

func (s *APIServer) listPlugins(ctx iris.Context) {
	// No need to lock.

	plugins := s._listPlugins()

	buff, err := json.Marshal(plugins)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", plugins, err))
	}

	ctx.Write(buff)
}

func (s *APIServer) listPluginTypes(ctx iris.Context) {
	buff, err := json.Marshal(plugins.PLUGIN_TYPES)
	if err != nil {
		panic(fmt.Errorf("marshal %s to json failed: %v",
			plugins.PLUGIN_TYPES, err))
	}

	ctx.Write(buff)
}
