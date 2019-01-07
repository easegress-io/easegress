package api

import (
	"github.com/kataras/iris"
	"github.com/megaease/easegateway/pkg/store"
)

func pluginAPIs(s *APIServer) []apiEntry {
	return []apiEntry{
		{
			path:    "/plugins",
			method:  "POST",
			handler: s.createPlugin,
		},
		{
			path:    "/plugins/{name:string}",
			method:  "PUT",
			handler: s.updatePlugin,
		},
		{
			path:    "/plugins/{name:string}",
			method:  "GET",
			handler: s.getPlugin,
		},
		{
			path:    "/plugins/{name:string}",
			method:  "DELETE",
			handler: s.deletePlugin,
		},
		{
			path:    "/plugins",
			method:  "GET",
			handler: s.listPlugins,
		},
		{
			path:    "/plugin-types",
			method:  "GET",
			handler: s.listPluginTypes,
		},
	}
}

func (s *APIServer) createPlugin(ctx iris.Context) {
	spec := new(store.PluginSpec)
	err := ctx.ReadJSON(spec)
	if err != nil {
		handlAPIError(ctx, iris.StatusBadRequest, err)
		return
	}

	err = s.store.CreatePlugin(spec)
	if err != nil {
		handlAPIError(ctx, iris.StatusBadRequest, err)
		return
	}
}

func (s *APIServer) deletePlugin(ctx iris.Context) {
	name := ctx.Params().Get("name")

	err := s.store.DeletePlugin(name)
	if err != nil {
		handlAPIError(ctx, iris.StatusBadRequest, err)
		return
	}
}

func (s *APIServer) getPlugin(ctx iris.Context)       {}
func (s *APIServer) updatePlugin(ctx iris.Context)    {}
func (s *APIServer) listPlugins(ctx iris.Context)     {}
func (s *APIServer) listPluginTypes(ctx iris.Context) {}
