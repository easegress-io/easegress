package api

import (
	"fmt"
	"sort"

	"github.com/kataras/iris"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/v"
	yaml "gopkg.in/yaml.v2"
)

const (
	// MetadataPrefix is the metadata prefix.
	MetadataPrefix = "/metadata"

	// ObjectMetadataPrefix is the object metadata prefix.
	ObjectMetadataPrefix = "/metadata/objects"

	// PluginMetaPrefix is the plugin of HTTPPipeline metadata prefix.
	PluginMetaPrefix = "/metadata/objects/httppipeline/plugins"
)

var (
	pluginBook  = map[string]*httppipeline.PluginRecord{}
	pluginKinds []string
)

func (s *Server) setupMetadaAPIs() {
	pluginBook = httppipeline.GetPluginBook()
	for kind, pr := range pluginBook {
		pluginBook[kind] = pr
		pluginKinds = append(pluginKinds, kind)
		sort.Strings(pr.Results)
	}
	sort.Strings(pluginKinds)

	metadataAPIs := make([]*apiEntry, 0)
	metadataAPIs = append(metadataAPIs,
		&apiEntry{
			Path:    PluginMetaPrefix,
			Method:  "GET",
			Handler: s.listPlugins,
		},
		&apiEntry{
			Path:    PluginMetaPrefix + "/{kind:string}" + "/description",
			Method:  "GET",
			Handler: s.getPluginDescription,
		},
		&apiEntry{
			Path:    PluginMetaPrefix + "/{kind:string}" + "/schema",
			Method:  "GET",
			Handler: s.getPluginSchema,
		},
		&apiEntry{
			Path:    PluginMetaPrefix + "/{kind:string}" + "/results",
			Method:  "GET",
			Handler: s.getPluginResults,
		},
	)

	s.apis = append(s.apis, metadataAPIs...)
}

func (s *Server) listPlugins(ctx iris.Context) {
	buff, err := yaml.Marshal(pluginKinds)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to yaml failed: %v", pluginKinds, err))
	}

	ctx.Header("Content-Type", "text/vnd.yaml")
	ctx.Write(buff)
}

func (s *Server) getPluginDescription(ctx iris.Context) {
	kind := ctx.Params().Get("kind")

	pr, exits := pluginBook[kind]
	if !exits {
		handleAPIError(ctx, iris.StatusNotFound, fmt.Errorf("not found"))
		return
	}

	ctx.WriteString(pr.Description)
}

func (s *Server) getPluginSchema(ctx iris.Context) {

	kind := ctx.Params().Get("kind")

	pr, exits := pluginBook[kind]
	if !exits {
		handleAPIError(ctx, iris.StatusNotFound, fmt.Errorf("not found"))
		return
	}

	buff, err := v.GetSchemaInYAML(pr.SpecType)
	if err != nil {
		panic(fmt.Errorf("get schema for %v failed: %v", pr.Kind, err))
	}

	ctx.Header("Content-Type", "text/vnd.yaml")
	ctx.Write(buff)
}

func (s *Server) getPluginResults(ctx iris.Context) {
	kind := ctx.Params().Get("kind")

	pr, exits := pluginBook[kind]
	if !exits {
		handleAPIError(ctx, iris.StatusNotFound, fmt.Errorf("not found"))
		return
	}

	buff, err := yaml.Marshal(pr.Results)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to yaml failed: %v", pr.Results, err))
	}

	ctx.Header("Content-Type", "text/vnd.yaml")
	ctx.Write(buff)
}
