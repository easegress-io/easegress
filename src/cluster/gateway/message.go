package gateway

import (
	"config"
)

const (
	// Requests/Responses need 1 byte header specifying message type.
	queryMaxSeqMessage MessageType = iota
	// Operation here means those operations for updating config,
	// We didn't choose the word `update` in order to makes naming clear.
	operationMessage
	retrieveMessage
	statMessage
	syncOPLogMessage
)

type MessageType uint8

// NOTICE: The type of all generic fields whose types are limited/known can be []byte.
// The []byte whose first-byte(header) represent the concrete type,
// the rest of it can be Unpacked to the corresponding type definitely.
// So interface{} is used to represent unlimited/unknown type.

// queryMaxSeqMessage
// No const for queryMaxSeqMessage for now.

type (
	// Pack Header: queryMaxSeqMessage
	ReqQueryMaxSeq struct{}
	// Pack Header: queryMaxSeqMessage
	RespQueryMaxSeq uint64
)

// operationMessage
const (
	createPlugin OperationType = iota
	updatePlugin
	deletePlugin
	createPipeline
	updatePipeline
	deletePipeline
)

type (
	OperationType uint8
	// Pack Header: operationMessage
	ReqOperation struct {
		Operation Operation
	}
	// Pack Header: operationMessage
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
	// Pack Header: createPlugin
	ContentCreatePlugin struct {
		Type   string
		Config interface{}
	}
	// Pack Header: updatePlugin
	ContentUpdatePlugin struct {
		Type   string
		Config interface{}
	}
	// Pack Header: deletePlugin
	ContentDeletePlugin struct {
		Name string
	}
	// Pack Header: createPipeline
	ContentCreatePipeline struct {
		Type   string
		Config interface{}
	}
	// Pack Header: updatePipeline
	ContentUpdatePipeline struct {
		Type   string
		Config interface{}
	}
	// Pack Header: deletePipeline
	ContentDeletePipeline struct {
		Name string
	}
)

// retrieveMessage
const (
	// NOTICE: Cluster REST API could use plural retrieval to implement single one.
	retrievePlugins RetrieveType = iota
	retrievePipelines
	retrievePluginTypes
	retrievePipelineTypes
)

type (
	RetrieveType uint8
	// Pack Header: retrieveMessage
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
	// Pack Header: retrieveMessage
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
	// Pack Header: retrievePlugins
	ResultRetrievePlugins struct {
		Plugins []config.PluginSpec
	}
	// Pack Header: retrievePipelines
	ResultRetrievePipelines struct {
		Pipelines []config.PipelineSpec
	}
	// Pack Header: retrievePluginTypes
	ResultRetrievePluginTypes struct {
		PluginTypes []string
	}
	// Pack Header: retrievePipelineTypes
	ResultRetrievePipelineTypes struct {
		PipelineTypes []string
	}
	// Pack Header: retrievePlugins
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

// statMessage
const (
	statPipelineIndicatorNames StatType = iota
	statPipelineIndicatorValue
	statPipelineIndicatorDesc
	statPluginIndicatorNames
	statPluginIndicatorValue
	statPluginIndicatorDesc
	statTaskIndicatorNames
	statTaskIndicatorValue
	statTaskIndicatorDesc
)

type (
	StatType uint8
	// Pack Header: statMessage
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
	// Pack Header: statPipelineIndicatorNames
	ResultStatPipelineIndicatorNames struct {
		Names []string
	}
	// Pack Header: statPipelineIndicatorValue
	ResultStatPipelineIndicatorValue struct {
		Value interface{}
	}
	// Pack Header: statPipelineIndicatorDesc
	ResultStatPipelineIndicatorDesc struct {
		Desc interface{}
	}
	// Pack Header: statPluginIndicatorNames
	ResultStatPluginIndicatorNames struct {
		Names []string
	}
	// Pack Header: statPluginIndicatorValue
	ResultStatPluginIndicatorValue struct {
		Value interface{}
	}
	// Pack Header: statPluginIndicatorDesc
	ResultStatPluginIndicatorDesc struct {
		Desc interface{}
	}
	// Pack Header: statTaskIndicatorNames
	ResultStatTaskIndicatorNames struct {
		Names []string
	}
	// Pack Header: statTaskIndicatorValue
	ResultStatTaskIndicatorValue struct {
		Value interface{}
	}
	// Pack Header: statTaskIndicatorDesc
	ResultStatTaskIndicatorDesc struct {
		Desc interface{}
	}
	// Pack Header: statPipelineIndicatorNames
	FilterPipelineIndicatorNames struct {
		PipelineName string
	}
	// Pack Header: statPipelineIndicatorValue
	FilterPipelineIndicatorValue struct {
		PipelineName  string
		IndicatorName string
	}
	// Pack Header: statPipelineIndicatorDesc
	FilterPipelineIndicatorDesc struct {
		PipelineName  string
		IndicatorName string
	}
	// Pack Header: statPluginIndicatorNames
	FilterPluginIndicatorNames struct {
		PluginName string
	}
	// Pack Header: statPluginIndicatorValue
	FilterPluginIndicatorValue struct {
		PluginName    string
		IndicatorName string
	}
	// Pack Header: statPluginIndicatorDesc
	FilterPluginIndicatorDesc struct {
		PluginName    string
		IndicatorName string
	}
	// Pack Header: statTaskIndicatorNames
	FilterTaskIndicatorNames struct {
		PipelineName string
	}
	// Pack Header: statTaskIndicatorValue
	FilterTaskIndicatorValue struct {
		PipelineName  string
		IndicatorName string
	}
	// Pack Header: statTaskIndicatorDesc
	FilterTaskIndicatorDesc struct {
		PipelineName  string
		IndicatorName string
	}
	// FIXME: Is it necessary or meaningful to add statistics of
	// uptime, rusage, loadavg of a group.
)

// syncOPLogMessage
// const for Operation in syncOPLogMessage is the same with
// corresponding stuff in operationMessage.
type (
	// Header: syncOPLogMessage.
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
