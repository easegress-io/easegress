package gateway

import "config"

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

type MessageType uint8

// MessageErr
const (
	ErrWrongFormat MessageErrType = iota
	ErrInternalServer
	ErrRetrieveInconsistency
	ErrRetrieveTimeout
)

type (
	MessageErrType uint8
	MessageErr     struct {
		Type MessageErrType
		Msg  string
	}
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
		Operation Operation
	}
	// Pack Header: operationMessage | operationRelayMessage
	RespOperation struct {
		Err *MessageErr
	}
	Operation struct {
		SeqBased uint64

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
		// RetrieveAllNodes is the flag to specify the write_mode node
		// retrieve just its own stuff then return immediately when false,
		// or retrieve corresponding stuff of all nodes in the group then
		// return when true. If any one of nodes has different stuff,
		// that would cause returning inconsistent error to the client.
		// The mechanism guarantees that retrieval must choose either
		// Consistency or Availability.
		RetrieveAllNodes bool

		// Below Filter* is Packed from corresponding struct
		FilterRetrievePlugins       *FilterRetrievePlugins
		FilterRetrievePipelines     *FilterRetrievePipelines
		FilterRetrievePluginTypes   *FilterRetrievePluginTypes
		FilterRetrievePipelineTypes *FilterRetrievePipelineTypes
	}
	// Pack Header: retrieveMessage | retrieveRelayMessage
	RespRetrieve struct {
		Err *MessageErr

		ResultRetrievePlugins       []byte // json
		ResultRetrievePipelines     []byte // json
		ResultRetrievePluginTypes   []byte // json
		ResultRetrievePipelineTypes []byte // json
	}
	FilterRetrievePlugins struct {
		NamePattern string
		Types       []string
	}
	FilterRetrievePipelines struct {
		NamePattern string
		Types       []string
	}
	FilterRetrievePluginTypes   struct{}
	FilterRetrievePipelineTypes struct{}
	ResultRetrievePlugins       struct {
		Plugins []config.PluginSpec `json:"plugins"`
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
		Err *MessageErr

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
		PluginName string
	}
	FilterPluginIndicatorValue struct {
		PluginName    string
		IndicatorName string
	}
	FilterPluginIndicatorDesc struct {
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
	// TODO: add uptime, rusage, loadavg of a group.
)

// opLogPullMessage
type (
	// Pack Header: opLogPullMessage
	ReqPullOPLog struct {
		LocalMaxSeq uint64
		WantMaxSeq  uint64
	}
	// Pack Header: opLogPullMessage
	RespPullOPLog struct {
		// It's recommended to check sequence of first operation and len
		// to get max sequence of SequentialOperations then just land
		// and record needed operations to local Operation Log.
		SequentialOperations []Operation
	}
)
