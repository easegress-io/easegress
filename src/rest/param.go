package rest

import (
	"config"
	"time"
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
