package rest

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	"config"

	"github.com/ant0ine/go-json-rest/rest"
)

//
// Admin API
//

type pluginCreationRequest struct {
	Type   string      `json:"type"`
	Config interface{} `json:"config"`
}

type pluginsRetrieveRequest struct {
	NamePattern string   `json:"name_pattern,omitempty"`
	Types       []string `json:"types"`
}

type pluginsRetrieveResponse struct {
	Plugins []config.PluginSpec `json:"plugins"`
}

type pluginUpdateRequest struct {
	Type   string      `json:"type"`
	Config interface{} `json:"config"`
}

////

type pipelineCreationRequest struct {
	Type   string      `json:"type"`
	Config interface{} `json:"config"`
}

type pipelinesRetrieveRequest struct {
	NamePattern string   `json:"name_pattern,omitempty"`
	Types       []string `json:"types"`
}

type pipelinesRetrieveResponse struct {
	Pipelines []config.PipelineSpec `json:"pipelines"`
}

type pipelineUpdateRequest struct {
	Type   string      `json:"type"`
	Config interface{} `json:"config"`
}

////

type pluginTypesRetrieveResponse struct {
	PluginTypes []string `json:"plugin_types"`
}

type pipelineTypesRetrieveResponse struct {
	PipelineTypes []string `json:"pipeline_types"`
}

//
// Statistics API
//

type IndicatorNamesRetrieveResponse struct {
	Names []string `json:"names"`
}

type IndicatorValueRetrieveResponse struct {
	Value interface{} `json:"value"`
}

type IndicatorDescriptionRetrieveResponse struct {
	Description interface{} `json:"desc"`
}

////

type GatewayUpTimeRetrieveResponse struct {
	UpTime time.Duration `json:"up_nanosec"`
}

// group is required, sync_all(default: false) and
// timeout(default: 30s, min: 10s) is optional
// e.g. /group_NY/foo?sync_all=false&timeout=30s
func parseClusterParam(r *rest.Request) (group string, syncAll bool, timeout time.Duration,
	err error) {
	group, err = url.QueryUnescape(r.PathParam("group"))
	if err != nil || len(group) == 0 {
		err = fmt.Errorf("invalid group")
		return
	}

	err = r.ParseForm()
	if err != nil {
		return
	}

	syncAllValue := r.Form.Get("sync_all")
	if len(syncAllValue) > 0 {
		syncAll, err = strconv.ParseBool(syncAllValue)
		if err != nil {
			err = fmt.Errorf("invalid sync_all")
			return
		}
	}

	timeoutValue := r.Form.Get("timeout")
	if len(timeoutValue) > 0 {
		timeout, err = time.ParseDuration(timeoutValue)
		if err != nil {
			err = fmt.Errorf("invalid timeout")
			return
		}
		minTimeout := time.Duration(10 * time.Second)
		if timeout < minTimeout {
			err = fmt.Errorf("timeout less than the minimum timeout %s", minTimeout)
			return
		}
	} else {
		timeout = 30 * time.Second
	}

	err = nil // for emphasizing
	return
}
