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

	// FilterMetaPrefix is the filter of HTTPPipeline metadata prefix.
	FilterMetaPrefix = "/metadata/objects/httppipeline/filters"
)

var (
	filterBook  = map[string]*httppipeline.FilterRecord{}
	filterKinds []string
)

func (s *Server) setupMetadaAPIs() {
	filterBook = httppipeline.GetFilterBook()
	for kind, pr := range filterBook {
		filterBook[kind] = pr
		filterKinds = append(filterKinds, kind)
		sort.Strings(pr.Results)
	}
	sort.Strings(filterKinds)

	metadataAPIs := make([]*apiEntry, 0)
	metadataAPIs = append(metadataAPIs,
		&apiEntry{
			Path:    FilterMetaPrefix,
			Method:  "GET",
			Handler: s.listFilters,
		},
		&apiEntry{
			Path:    FilterMetaPrefix + "/{kind:string}" + "/description",
			Method:  "GET",
			Handler: s.getFilterDescription,
		},
		&apiEntry{
			Path:    FilterMetaPrefix + "/{kind:string}" + "/schema",
			Method:  "GET",
			Handler: s.getFilterSchema,
		},
		&apiEntry{
			Path:    FilterMetaPrefix + "/{kind:string}" + "/results",
			Method:  "GET",
			Handler: s.getFilterResults,
		},
	)

	s.apis = append(s.apis, metadataAPIs...)
}

func (s *Server) listFilters(ctx iris.Context) {
	buff, err := yaml.Marshal(filterKinds)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to yaml failed: %v", filterKinds, err))
	}

	ctx.Header("Content-Type", "text/vnd.yaml")
	ctx.Write(buff)
}

func (s *Server) getFilterDescription(ctx iris.Context) {
	kind := ctx.Params().Get("kind")

	pr, exits := filterBook[kind]
	if !exits {
		handleAPIError(ctx, iris.StatusNotFound, fmt.Errorf("not found"))
		return
	}

	ctx.WriteString(pr.Description)
}

func (s *Server) getFilterSchema(ctx iris.Context) {

	kind := ctx.Params().Get("kind")

	pr, exits := filterBook[kind]
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

func (s *Server) getFilterResults(ctx iris.Context) {
	kind := ctx.Params().Get("kind")

	pr, exits := filterBook[kind]
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
