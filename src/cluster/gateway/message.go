package gateway

import (
	"config"
	"time"
)

// MessageType
const (
	// Requests/Responses need 1 byte header specifying message type.

	queryGroupMaxSeqMessage MessageType = iota

	// Operation here means those operations for updating config,
	// We didn't choose the word `update` in order to makes naming clear.
	operationMessage
	operationRelayMessage

	retrieveMessage
	retrieveRelayMessage

	statMessage
	statRelayMessage

	opLogPullMessage
)

type (
	MessageType uint8
)

// queryGroupMaxSeqMessage

type (
	// Pack Header: queryGroupMaxSeqMessage
	ReqQueryGroupMaxSeq struct{}
	// Pack Header: queryGroupMaxSeqMessage
	RespQueryGroupMaxSeq uint64
)

// operationMessage | operationRelayMessage

type (
	// Pack Header: operationMessage | operationRelayMessage
	ReqOperation struct {
		OperateAllNodes bool
		Timeout         time.Duration
		StartSeq        uint64
		Operation       *Operation
	}
	// Pack Header: operationMessage | operationRelayMessage
	RespOperation struct {
		Err *ClusterError
	}
	Operation struct {
		ContentCreatePlugin   *ContentCreatePlugin
		ContentUpdatePlugin   *ContentUpdatePlugin
		ContentDeletePlugin   *ContentDeletePlugin
		ContentCreatePipeline *ContentCreatePipeline
		ContentUpdatePipeline *ContentUpdatePipeline
		ContentDeletePipeline *ContentDeletePipeline
	}
	ContentCreatePlugin struct {
		Type   string
		Config []byte // json
	}
	ContentUpdatePlugin struct {
		Type   string
		Config []byte // json
	}
	ContentDeletePlugin struct {
		Name string
	}
	ContentCreatePipeline struct {
		Type   string
		Config []byte // json
	}
	ContentUpdatePipeline struct {
		Type   string
		Config []byte // json
	}
	ContentDeletePipeline struct {
		Name string
	}
)

// retrieveMessage | retrieveRelayMessage
type (
	// Pack Header: retrieveMessage | retrieveRelayMessage
	ReqRetrieve struct {
		Timeout time.Duration

		// RetrieveAllNodes is the flag to represent if the node under write mode
		// retrieves just its own stuff (false value) or retrieves corresponding stuff
		// of all nodes in the group (true value). If any one of nodes has different stuff,
		// that would cause returning inconsistent error to the client.
		// The mechanism guarantees that retrieval must choose either
		// Consistency or Availability.
		RetrieveAllNodes bool

		// Below Filter* is Packed from corresponding struct
		FilterRetrievePlugin        *FilterRetrievePlugin
		FilterRetrievePlugins       *FilterRetrievePlugins
		FilterRetrievePipeline      *FilterRetrievePipeline
		FilterRetrievePipelines     *FilterRetrievePipelines
		FilterRetrievePluginTypes   *FilterRetrievePluginTypes
		FilterRetrievePipelineTypes *FilterRetrievePipelineTypes
	}
	// Pack Header: retrieveMessage | retrieveRelayMessage
	RespRetrieve struct {
		Err *ClusterError

		ResultRetrievePlugin        []byte // json
		ResultRetrievePlugins       []byte // json
		ResultRetrievePipeline      []byte // json
		ResultRetrievePipelines     []byte // json
		ResultRetrievePluginTypes   []byte // json
		ResultRetrievePipelineTypes []byte // json
	}
	FilterRetrievePlugin struct {
		Name string
	}
	FilterRetrievePlugins struct {
		NamePattern string
		Types       []string
	}
	FilterRetrievePipeline struct {
		Name string
	}
	FilterRetrievePipelines struct {
		NamePattern string
		Types       []string
	}
	FilterRetrievePluginTypes   struct{}
	FilterRetrievePipelineTypes struct{}
	ResultRetrievePlugin        struct {
		Plugin config.PluginSpec `json:"plugin"`
	}
	ResultRetrievePlugins struct {
		Plugins []config.PluginSpec `json:"plugins"`
	}
	ResultRetrievePipeline struct {
		Pipeline config.PipelineSpec `json:"pipeline"`
	}
	ResultRetrievePipelines struct {
		Pipelines []config.PipelineSpec `json:"pipelines"`
	}
	ResultRetrievePluginTypes struct {
		PluginTypes []string `json:"plugin_types"`
	}
	ResultRetrievePipelineTypes struct {
		PipelineTypes []string `json:"pipeline_types"`
	}
)

// statMessage | statRelayMessage
type (
	// Pack Header: statMessage | statRelayMessage
	ReqStat struct {
		Timeout time.Duration

		FilterPipelineIndicatorNames *FilterPipelineIndicatorNames
		FilterPipelineIndicatorValue *FilterPipelineIndicatorValue
		FilterPipelineIndicatorDesc  *FilterPipelineIndicatorDesc
		FilterPluginIndicatorNames   *FilterPluginIndicatorNames
		FilterPluginIndicatorValue   *FilterPluginIndicatorValue
		FilterPluginIndicatorDesc    *FilterPluginIndicatorDesc
		FilterTaskIndicatorNames     *FilterTaskIndicatorNames
		FilterTaskIndicatorValue     *FilterTaskIndicatorValue
		FilterTaskIndicatorDesc      *FilterTaskIndicatorDesc
	}
	// Pack Header: statMessage | statRelayMessage
	RespStat struct {
		Err *ClusterError

		Names []byte // json
		Value []byte // json
		Desc  []byte // json
	}
	FilterPipelineIndicatorNames struct {
		PipelineName string
	}
	FilterPipelineIndicatorValue struct {
		PipelineName  string
		IndicatorName string
	}
	FilterPipelineIndicatorDesc struct {
		PipelineName  string
		IndicatorName string
	}
	FilterPluginIndicatorNames struct {
		PipelineName string
		PluginName   string
	}
	FilterPluginIndicatorValue struct {
		PipelineName  string
		PluginName    string
		IndicatorName string
	}
	FilterPluginIndicatorDesc struct {
		PipelineName  string
		PluginName    string
		IndicatorName string
	}
	FilterTaskIndicatorNames struct {
		PipelineName string
	}
	FilterTaskIndicatorValue struct {
		PipelineName  string
		IndicatorName string
	}
	FilterTaskIndicatorDesc struct {
		PipelineName  string
		IndicatorName string
	}
	ResultStatIndicatorNames struct {
		Names []string `json:"names"`
	}
	ResultStatIndicatorValue struct {
		Value interface{} `json:"value"`
	}
	ResultStatIndicatorDesc struct {
		Desc interface{} `json:"desc"`
	}
	// TODO: add health stuff, including uptime, rusage, loadavg of a group.
)

// opLogPullMessage
type (
	// Pack Header: opLogPullMessage
	ReqOPLogPull struct {
		StartSeq   uint64
		CountLimit uint64
	}
	// Pack Header: opLogPullMessage
	RespOPLogPull struct {
		StartSeq             uint64
		SequentialOperations []*Operation
	}
)
