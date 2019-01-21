package api

import (
	"encoding/json"
	"fmt"

	"github.com/megaease/easegateway/pkg/stat"

	"github.com/kataras/iris"
)

func (s *APIServer) setupStatAPIs() {
	statAPIs := []*apiEntry{
		{
			Path:    "/stats",
			Method:  "GET",
			Handler: s.listStat,
		},
	}
	s.apis = append(s.apis, statAPIs...)
}

type (
	ListStatResp struct {
		//	pipelineName memberName
		Values map[string]map[string]stat.PipelineStat `json:"values"`
	}
)

func (s *APIServer) listStat(ctx iris.Context) {
	values := s._listStats()

	resp := ListStatResp{
		Values: values,
	}

	buff, err := json.Marshal(resp)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", resp, err))
	}

	ctx.Write(buff)
}
