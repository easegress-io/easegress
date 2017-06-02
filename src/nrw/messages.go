package nrw

import (
	"config"
)

// MessageType
const (
	// Requests/Responses need 1 byte header specifying message type.
	MessageQueryMaxSeq MessageType = iota
	// Operation here means those operations for updating config,
	// We didn't choose the word `update` in order to makes naming clear.
	MessageOperation
	MessageRetrieve
	MessasgeStat
	MessageSyncOPLog
)

type MessageType uint8

// NOTICE: The type of all generic fileds whose types are limieted/known can be []byte.
// The []byte whose first-byte(header) represent the concrete type,
// the rest of it can be Unpacked to the corresponding type definitely.
// So interface{} is used to represent unlimited/unknown type.

// MessageQueryMaxSeq
// No const for MessageQueryMaxSeq for now.

type (
	// Pack Header: MessageQueryMaxSeq
	ReqQueryMaxSeq struct{}
	// Pack Header: MessageQueryMaxSeq
	RespQueryMaxSeq uint64
)

// MessageOperation
const (
	OperationCreatePlugin OperationType = iota
	OperationUpdatePlugin
	OperationDeletePlugin
	OperationCreatePipeline
	OperationUpdatePipeline
	OperationDeletePipeline
)

type (
	OperationType uint8
	// Pack Header: MessageOperation
	ReqOperation struct {
		Operation Operation
	}
	// Pack Header: MessageOperation
	RespOperation struct {
		Err OperationErr // If Err is non-nil, the update operation failed owe to Err.
	}
	Operation struct {
		SeqBased uint64
		// Unpack Header: OperationType
		// Possible Type:
		//      ContentCreatePlugin, ContentUpdatePlugin, ContentDeletePlugin,
		//      ContentCreatePipeline, ContentUpdatePipeline, ContentDeletePipeline
		Content []byte
	}
	// Pack Header: OperationCreatePlugin
	ContentCreatePlugin struct {
		Type   string
		Config interface{}
	}
	// Pack Header: OperationUpdatePlugin
	ContentUpdatePlugin struct {
		Type   string
		Config interface{}
	}
	// Pack Header: OperationDeletePlugin
	ContentDeletePlugin struct {
		Name string
	}
	// Pack Header: OperationCreatePipeline
	ContentCreatePipeline struct {
		Type   string
		Config interface{}
	}
	// Pack Header: OperationUpdatePipeline
	ContentUpdatePipeline struct {
		Type   string
		Config interface{}
	}
	// Pack Header: OperationDeletePipeline
	ContentDeletePipeline struct {
		Name string
	}
)

// MessageRetrieve
const (
	// NOTICE: Cluster REST API could use plural retrieval to implement single one.
	RetrievePlugins RetrieveType = iota
	RetrievePipelines
	RetrievePluginTypes
	RetrievePipelineTypes
)

type (
	RetrieveType uint8
	// Pack Header: MessageRetrieve
	ReqRetrieve struct {
		// RetrieveAllNodes is the flag to specify the write_mode node
		// retrieve just its own stuff then return immediately when false,
		// or retrieve corresponding stuff of all nodes in the group then
		// return when true. If any one of nodes has different stuff,
		// that would cause returning inconsistent error to the client.
		// The mechanism guarantees that retrieval must choose either
		// Consistency or Availability.
		RetrieveAllNodes bool
		// Unpack Header: RetrieveType
		// Possible Type:
		//      FilterRetrievePlugins, FilterRetrievePipelines
		// No filter for RetrievePluginTypes, RetrievePipelineTypes for now.
		Filter []byte
	}
	// Pack Header: MessageRetrieve
	RespRetrieve struct {
		// If Err is non-nil, the retrieval failed owe to Err,
		// and Result will be nil.
		Err RetrieveErr
		// Unpack Header: RetrieveType
		// Possible Type:
		//      ResultRetrievePlugins, ResultRetrievePipelines,
		//      ResultRetrievePluginTypes, ResultRetrievePipelineTypes
		Result []byte
	}
	// Pack Header: RetrievePlugins
	ResultRetrievePlugins struct {
		Plugins []config.PluginSpec
	}
	// Pack Header: RetrievePipelines
	ResultRetrievePipelines struct {
		Pipelines []config.PipelineSpec
	}
	// Pack Header: RetrievePluginTypes
	ResultRetrievePluginTypes struct {
		PluginTypes []string
	}
	// Pack Header: RetrievePipelineTypes
	ResultRetrievePipelineTypes struct {
		PipelineTypes []string
	}
	// Pack Header: RetrievePlugins
	FilterRetrievePlugins struct {
		NamePattern string
		Types       []string
	}
	// Pack Header: RetrievePipelines
	FilterRetrievePipelines struct {
		NamePattern string
		Types       []string
	}
)

// MessageStat
const (
	StatPipelineIndicatorNames StatType = iota
	StatPipelineIndicatorValue
	StatPipelineIndicatorDesc
	StatPluginIndicatorNames
	StatPluginIndicatorValue
	StatPluginIndicatorDesc
	StatTaskIndicatorNames
	StatTaskIndicatorValue
	StatTaskIndicatorDesc
)

type (
	StatType uint8
	// Pack Header: MessageStat
	ReqStat struct {
		// Unpack Header: StatType
		// Possible Type:
		//      FilterPipelineIndicatorNames, FilterPipelineIndicatorValue, FilterPipelineIndicatorDesc,
		//      FilterPluginIndicatorNames, FilterPluginIndicatorValue, FilterPluginIndicatorDesc,
		//      FilterTaskIndicatorNames, FilterTaskIndicatorValue, FilterTaskIndicatorDesc
		Filter []byte
	}
	RespStat struct {
		// If Err is non-nil, the stat failed owe to Err,
		// and Result will be nil.
		Err StatErr
		// Unpack Header: StatType
		// Possible Type:
		//      ResultStatPipelineIndicatorNames, ResultStatPipelineIndicatorValue, ResultStatPipelineIndicatorDesc,
		//      ResultStatPluginIndicatorNames, ResultStatPluginIndicatorValue, ResultStatPluginIndicatorDesc,
		//      ResultStatTaskIndicatorNames, ResultStatTaskIndicatorValue, ResultStatTaskIndicatorDesc
		Result []byte
	}
	// Pack Header: StatPipelineIndicatorNames
	ResultStatPipelineIndicatorNames struct {
		Names []string
	}
	// Pack Header: StatPipelineIndicatorValue
	ResultStatPipelineIndicatorValue struct {
		Value interface{}
	}
	// Pack Header: StatPipelineIndicatorDesc
	ResultStatPipelineIndicatorDesc struct {
		Desc interface{}
	}
	// Pack Header: StatPluginIndicatorNames
	ResultStatPluginIndicatorNames struct {
		Names []string
	}
	// Pack Header: StatPluginIndicatorValue
	ResultStatPluginIndicatorValue struct {
		Value interface{}
	}
	// Pack Header: StatPluginIndicatorDesc
	ResultStatPluginIndicatorDesc struct {
		Desc interface{}
	}
	// Pack Header: StatTaskIndicatorNames
	ResultStatTaskIndicatorNames struct {
		Names []string
	}
	// Pack Header: StatTaskIndicatorValue
	ResultStatTaskIndicatorValue struct {
		Value interface{}
	}
	// Pack Header: StatTaskIndicatorDesc
	ResultStatTaskIndicatorDesc struct {
		Desc interface{}
	}
	// Pack Header: StatPipelineIndicatorNames
	FilterPipelineIndicatorNames struct {
		PipelineName string
	}
	// Pack Header: StatPipelineIndicatorValue
	FilterPipelineIndicatorValue struct {
		PipelineName  string
		IndicatorName string
	}
	// Pack Header: StatPipelineIndicatorDesc
	FilterPipelineIndicatorDesc struct {
		PipelineName  string
		IndicatorName string
	}
	// Pack Header: StatPluginIndicatorNames
	FilterPluginIndicatorNames struct {
		PluginName string
	}
	// Pack Header: StatPluginIndicatorValue
	FilterPluginIndicatorValue struct {
		PluginName    string
		IndicatorName string
	}
	// Pack Header: StatPluginIndicatorDesc
	FilterPluginIndicatorDesc struct {
		PluginName    string
		IndicatorName string
	}
	// Pack Header: StatTaskIndicatorNames
	FilterTaskIndicatorNames struct {
		PipelineName string
	}
	// Pack Header: StatTaskIndicatorValue
	FilterTaskIndicatorValue struct {
		PipelineName  string
		IndicatorName string
	}
	// Pack Header: StatTaskIndicatorDesc
	FilterTaskIndicatorDesc struct {
		PipelineName  string
		IndicatorName string
	}
	// FIXME: Is it necessary or meaningful to add statistics of
	// uptime, rusage, loadavg of a group.
)

// MessageSyncOPLog
// const for Opertion in MessageSyncOPLog is the same with
// corresponding stuff in MessageOperation.
type (
	// Header: MessageSyncOPLog.
	ReqSyncOPLog struct {
		LocalMaxSeq uint64
		WantMaxSeq  uint64
	}
	RespSyncOPLog struct {
		// It's recommended to check sequence of first operation and len
		// to get max sequence of SequentialOperations then just land
		// and record needed operations to local Operation Log.
		SequentialOperations []Operation
	}
)
