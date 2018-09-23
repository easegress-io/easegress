package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/hexdecteam/easegateway/pkg/common"
	"github.com/hexdecteam/easegateway/pkg/config"
	"github.com/hexdecteam/easegateway/pkg/logger"
	"github.com/hexdecteam/easegateway/pkg/model"
	"github.com/hexdecteam/easegateway/pkg/plugins"

	"github.com/ugorji/go/codec"
)

// MessageType
const (
	// Requests/Responses need 1 byte header specifying message type.

	querySeqMessage MessageType = iota

	// Operation here means those operations for updating config,
	// We didn't choose the word `update` in order to makes naming clear.
	operationMessage
	operationRelayMessage

	// Retrieve configurations
	retrieveMessage
	retrieveRelayMessage

	// Retrieve statistics
	statMessage
	statRelayMessage

	opLogPullMessage

	// Query cluster information
	queryMemberMessage
	queryMembersListMessage
	queryGroupMessage
)

type MessageType uint8

// ReqQuerySeq
// Pack Header: querySeqMessage
type (
	ReqQuerySeq  struct{}
	RespQuerySeq struct {
		Min uint64
		Max uint64
	}
)

// ReqOperation
// Pack Header: operationMessage | operationRelayMessage
type (
	ReqOperation struct {
		OperateAllNodes bool
		Timeout         time.Duration
		StartSeq        uint64
		Operation       *Operation
	}
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

func validPluginConfig(typ string, byt []byte) (bool, error) {
	conf, err := plugins.GetConfig(typ)
	if err != nil {
		return false, fmt.Errorf("Get plugin type %s config failed: %v", typ, err)
	}
	if err = json.Unmarshal(byt, conf); err != nil {
		return false, fmt.Errorf("Unmarshal plugin config for type %s failed: %v", typ, err)
	}

	return true, nil
}

func validPipelineConfig(typ string, byt []byte) (bool, error) {
	conf, err := model.GetPipelineConfig(typ)
	if err != nil {
		return false, fmt.Errorf("Get pipeline type %s config failed: %v", typ, err)
	}
	if err = json.Unmarshal(byt, conf); err != nil {
		return false, fmt.Errorf("Unmarshal pipeline config for type %s failed: %v", typ, err)
	}
	return true, nil
}

// Operation includes marshalled plugin configuration in json format,
// we use that directly to prepare human readable json representation of the operation
func (op *Operation) ToHumanReadableJSON() ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	buf.WriteString(`{`)

	if op.ContentCreatePlugin != nil {
		if valid, err := validPluginConfig(op.ContentCreatePlugin.Type, op.ContentCreatePlugin.Config); !valid {
			return nil, err
		}
		buf.WriteString(fmt.Sprintf(`"CreatePlugin": {"Type": "%s", "Config": %s}`, op.ContentCreatePlugin.Type, op.ContentCreatePlugin.Config))
	}
	if op.ContentUpdatePlugin != nil {
		if valid, err := validPluginConfig(op.ContentUpdatePlugin.Type, op.ContentUpdatePlugin.Config); !valid {
			return nil, err
		}
		buf.WriteString(fmt.Sprintf(`"UpdatePlugin": {"Type": "%s", "Config": %s}`, op.ContentUpdatePlugin.Type, op.ContentUpdatePlugin.Config))
	}
	if op.ContentDeletePlugin != nil {
		buf.WriteString(fmt.Sprintf(`"DeletePlugin": %s`, op.ContentDeletePlugin.Name))
	}
	if op.ContentCreatePipeline != nil {
		if valid, err := validPipelineConfig(op.ContentCreatePipeline.Type, op.ContentCreatePipeline.Config); !valid {
			return nil, err
		}
		buf.WriteString(fmt.Sprintf(`"CreatePipeline": {"Type": "%s", "Config": %s}`, op.ContentCreatePipeline.Type, op.ContentCreatePipeline.Config))
	}
	if op.ContentUpdatePipeline != nil {
		if valid, err := validPipelineConfig(op.ContentUpdatePipeline.Type, op.ContentUpdatePipeline.Config); !valid {
			return nil, err
		}
		buf.WriteString(fmt.Sprintf(`"UpdatePipeline": {"Type": "%s", "Config": %s}`, op.ContentUpdatePipeline.Type, op.ContentUpdatePipeline.Config))
	}
	if op.ContentDeletePipeline != nil {
		buf.WriteString(fmt.Sprintf(`"DeletePipeline": %s`, op.ContentDeletePipeline.Name))
	}
	buf.WriteString(`}`)
	return buf.Bytes(), nil
}

// ReqRetrieve
// Pack Header: retrieveMessage | retrieveRelayMessage
type (
	ReqRetrieve struct {
		Timeout time.Duration

		// RetrieveAllNodes is the flag to represent if the node under write mode
		// retrieves just its own stuff (false value) or retrieves corresponding stuff
		// of all nodes in the group (true value). If any one of nodes has different stuff,
		// that would cause returning inconsistent error to the client.
		// The mechanism guarantees that retrieval must choose either
		// Consistency or Availability.
		RetrieveAllNodes bool

		// Pagination fields which only leverages retrieving plugins/pipelines.
		// Both of page and limit start from 1.
		// The defaul value(page=0, limit=0) means (page=1, limit=5).
		// Notice it always returns empty result if one and only one
		// of page and limit is 0.
		Page  uint32
		Limit uint32

		// Below Filter* is Packed from corresponding struct
		FilterRetrievePlugin        *FilterRetrievePlugin
		FilterRetrievePlugins       *FilterRetrievePlugins
		FilterRetrievePipeline      *FilterRetrievePipeline
		FilterRetrievePipelines     *FilterRetrievePipelines
		FilterRetrievePluginTypes   *FilterRetrievePluginTypes
		FilterRetrievePipelineTypes *FilterRetrievePipelineTypes
	}
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

	paginationInfo struct {
		Total int    `json:"total"`
		Page  uint32 `json:"page"`
		Limit uint32 `json:"limit"`
	}

	ResultRetrievePlugin struct {
		Plugin config.PluginSpec `json:"plugin"`
	}
	ResultRetrievePlugins struct {
		Pagination paginationInfo      `json:"pagination"`
		Plugins    []config.PluginSpec `json:"plugins"`
	}
	ResultRetrievePipeline struct {
		Pipeline config.PipelineSpec `json:"pipeline"`
	}
	ResultRetrievePipelines struct {
		Pagination paginationInfo        `json:"pagination"`
		Pipelines  []config.PipelineSpec `json:"pipelines"`
	}
	ResultRetrievePluginTypes struct {
		PluginTypes []string `json:"plugin_types"`
	}
	ResultRetrievePipelineTypes struct {
		PipelineTypes []string `json:"pipeline_types"`
	}
)

// ReqOPLogPull
// Pack Header: opLogPullMessage
type (
	ReqOPLogPull struct {
		StartSeq   uint64
		CountLimit uint64
	}
	RespOPLogPull struct {
		StartSeq             uint64
		SequentialOperations []*Operation
	}
)

// StatType is only used to indicate the statistics related request types
type StatType uint8

const (
	// NoneStat can be deleted if msgpack solve this issue:
	// https://github.com/ugorji/go/issues/230
	// we need to encode/decode StatType first in RespStat
	NoneStat StatType = iota
	PipelineIndicatorNamesStat
	PipelineIndicatorValueStat
	PipelineIndicatorsValueStat
	PipelineIndicatorDescStat
	PluginIndicatorNamesStat
	PluginIndicatorValueStat
	PluginIndicatorDescStat
	TaskIndicatorNamesStat
	TaskIndicatorValueStat
	TaskIndicatorDescStat

	// dedicated response
	AggregatedIndicatorValueStat
	AggregatedIndicatorsValueStat
)

// Stater defines methods by which a statistics can valid it as a request
// and execute to retrieve the result
type Stater interface {
	// Execute is used to execute Stat and retrieve result
	Execute(memberName string, m *model.Model) (StatResult, error, ClusterErrorType)
	// Validate is used to check whether this stat is valid
	Validate() (error, ClusterErrorType)
}

// ReqStat
// Pack Header: statMessage | statRelayMessage
type ReqStat struct {
	Detail bool // It is Leveraged by value[s] filters.
	Stat   Stater
}

// ReqStat implements codec.Selfer interface, which decodes or encodes itself
// These two functions are used by codec.Decoder, codec.Encoder, so
// error handling comply to codec's style
func (r *ReqStat) CodecDecodeSelf(dec *codec.Decoder) {
	dec.MustDecode(&r.Detail)
	var typ StatType
	dec.MustDecode(&typ)
	var stat Stater
	switch typ {
	case PipelineIndicatorNamesStat:
		stat = &FilterPipelineIndicatorNames{}
	case PipelineIndicatorValueStat:
		stat = &FilterPipelineIndicatorValue{}
	case PipelineIndicatorsValueStat:
		stat = &FilterPipelineIndicatorsValue{}
	case PipelineIndicatorDescStat:
		stat = &FilterPipelineIndicatorDesc{}
	case PluginIndicatorNamesStat:
		stat = &FilterPluginIndicatorNames{}
	case PluginIndicatorValueStat:
		stat = &FilterPluginIndicatorValue{}
	case PluginIndicatorDescStat:
		stat = &FilterPluginIndicatorDesc{}
	case TaskIndicatorNamesStat:
		stat = &FilterTaskIndicatorNames{}
	case TaskIndicatorValueStat:
		stat = &FilterTaskIndicatorValue{}
	case TaskIndicatorDescStat:
		stat = &FilterTaskIndicatorDesc{}

	default:
		panic(fmt.Sprintf("unimplemented StatType: %v", typ))
	}
	dec.MustDecode(&stat)
	r.Stat = stat
}

func (r *ReqStat) CodecEncodeSelf(enc *codec.Encoder) {
	enc.MustEncode(&r.Detail)
	var typ StatType
	switch v := r.Stat.(type) {
	case *FilterPipelineIndicatorNames:
		typ = PipelineIndicatorNamesStat
	case *FilterPipelineIndicatorValue:
		typ = PipelineIndicatorValueStat
	case *FilterPipelineIndicatorsValue:
		typ = PipelineIndicatorsValueStat
	case *FilterPipelineIndicatorDesc:
		typ = PipelineIndicatorDescStat
	case *FilterPluginIndicatorNames:
		typ = PluginIndicatorNamesStat
	case *FilterPluginIndicatorValue:
		typ = PluginIndicatorValueStat
	case *FilterPluginIndicatorDesc:
		typ = PluginIndicatorDescStat
	case *FilterTaskIndicatorNames:
		typ = TaskIndicatorNamesStat
	case *FilterTaskIndicatorValue:
		typ = TaskIndicatorValueStat
	case *FilterTaskIndicatorDesc:
		typ = TaskIndicatorDescStat
	default:
		panic(fmt.Sprintf("unimplemented StatType: %v", reflect.TypeOf(v)))
	}

	enc.MustEncode(&typ)
	enc.MustEncode(&r.Stat)
}

// ReqStat filters
type FilterPipelineIndicatorNames struct {
	PipelineName string
}

func (f *FilterPipelineIndicatorNames) Execute(memberName string, mod *model.Model) (StatResult, error, ClusterErrorType) {
	// TODO(shengdong, zhiyan) statRegistry -> StatRegistry
	stat := mod.StatRegistry().GetPipelineStatistics(f.PipelineName)
	if stat == nil {
		return nil, fmt.Errorf("pipeline %s statistics not found", f.PipelineName), PipelineStatNotFoundError
	}

	r := new(ResultStatIndicatorNames)
	r.Names = stat.PipelineIndicatorNames()

	// returns with stable order
	sort.Strings(r.Names)
	return r, nil, NoneClusterError
}

func (f *FilterPipelineIndicatorNames) Validate() (error, ClusterErrorType) {
	if len(f.PipelineName) == 0 {
		return fmt.Errorf("empty pipeline name in filter to retireve " +
			"pipeline statistics indicator names"), InternalServerError
	}
	return nil, NoneClusterError
}

type FilterPipelineIndicatorValue struct {
	PipelineName  string
	IndicatorName string
}

func (f *FilterPipelineIndicatorValue) Execute(memberName string, mod *model.Model) (StatResult, error, ClusterErrorType) {
	stat := mod.StatRegistry().GetPipelineStatistics(f.PipelineName)
	if stat == nil {
		return nil, fmt.Errorf("pipeline %s statistics not found", f.PipelineName), PipelineStatNotFoundError
	}
	indicatorNames := stat.PipelineIndicatorNames()
	if !common.StrInSlice(f.IndicatorName, indicatorNames) {
		return nil, fmt.Errorf("indicator %s not found", f.IndicatorName),
			RetrievePipelineStatIndicatorNotFoundError
	}

	var err error
	r := &ResultStatIndicatorValue{
		MemberName: memberName,
		Name:       f.IndicatorName,
	}

	r.Name = f.IndicatorName
	r.Value, err = stat.PipelineIndicatorValue(f.IndicatorName)
	if err != nil {
		logger.Errorf("[retrieve the value of pipeline %s statistics indicator %s "+
			"from model failed: %v]", f.PipelineName, f.IndicatorName, err)
		return nil, fmt.Errorf("evaluate indicator %s value failed", f.IndicatorName),
			RetrievePipelineStatValueError
	}

	return r, nil, NoneClusterError
}

func (f *FilterPipelineIndicatorValue) Validate() (error, ClusterErrorType) {
	if len(f.PipelineName) == 0 {
		return fmt.Errorf("empty pipeline name in filter to retrieve " +
			"pipeline statistics indicator value"), InternalServerError
	}
	if len(f.IndicatorName) == 0 {
		return fmt.Errorf("empty indicator name in filter to " +
			"retrieve pipeline statistics indicator value"), InternalServerError
	}

	return nil, NoneClusterError
}

type FilterPipelineIndicatorsValue struct {
	PipelineName   string
	IndicatorNames []string
}

func (f *FilterPipelineIndicatorsValue) Execute(memberName string, mod *model.Model) (StatResult, error, ClusterErrorType) {
	stat := mod.StatRegistry().GetPipelineStatistics(f.PipelineName)
	if stat == nil {
		return nil, fmt.Errorf("pipeline %s statistics not found", f.PipelineName), PipelineStatNotFoundError
	}
	r := &ResultStatIndicatorsValue{
		MemberName: memberName,
	}

	// If specified indicator doesn't exist, its coordinate value in r.Values will be nil
	r.Values = stat.PipelineIndicatorsValue(f.IndicatorNames)

	return r, nil, NoneClusterError
}

func (f *FilterPipelineIndicatorsValue) Validate() (error, ClusterErrorType) {
	if len(f.PipelineName) == 0 {
		return fmt.Errorf("empty pipeline name in filter to retrieve " +
			"pipeline statistics indicators value"), InternalServerError
	}
	if len(f.IndicatorNames) == 0 {
		return fmt.Errorf("empty indicators name in filter to " +
			"retrieve pipeline statistics indicators value"), InternalServerError
	}
	for _, indicatorName := range f.IndicatorNames {
		if len(indicatorName) == 0 {
			return fmt.Errorf("empty indicator name in filter to " +
				"retrieve pipeline statistics indicator value"), InternalServerError
		}
	}

	return nil, NoneClusterError
}

type FilterPipelineIndicatorDesc struct {
	PipelineName  string
	IndicatorName string
}

func (f *FilterPipelineIndicatorDesc) Execute(memberName string, mod *model.Model) (StatResult, error, ClusterErrorType) {
	stat := mod.StatRegistry().GetPipelineStatistics(f.PipelineName)
	if stat == nil {
		return nil, fmt.Errorf("pipeline %s statistics not found", f.PipelineName), PipelineStatNotFoundError
	}
	indicatorNames := stat.PipelineIndicatorNames()
	if !common.StrInSlice(f.IndicatorName, indicatorNames) {
		return nil, fmt.Errorf("indicator %s not found", f.IndicatorName),
			RetrievePipelineStatIndicatorNotFoundError
	}

	var err error
	r := new(ResultStatIndicatorDesc)
	r.Desc, err = stat.PipelineIndicatorDescription(f.IndicatorName)
	if err != nil {
		logger.Errorf("[retrieve the description of pipeline %s statistics indicator %s "+
			"from model failed: %v]", f.PipelineName, f.IndicatorName, err)
		return nil, fmt.Errorf("describe indicator %s failed", f.IndicatorName),
			RetrievePipelineStatDescError
	}

	return r, nil, NoneClusterError
}

func (f *FilterPipelineIndicatorDesc) Validate() (error, ClusterErrorType) {
	if len(f.PipelineName) == 0 {
		return fmt.Errorf("empty pipeline name in filter to retrieve " +
			"pipeline statistics indicator description"), InternalServerError
	}
	if len(f.IndicatorName) == 0 {
		return fmt.Errorf("empty indicator name in filter to retrieve " +
			"pipeline statistics indicator description"), InternalServerError
	}

	return nil, NoneClusterError
}

type FilterPluginIndicatorNames struct {
	PipelineName string
	PluginName   string
}

func (f *FilterPluginIndicatorNames) Execute(memberName string, mod *model.Model) (StatResult, error, ClusterErrorType) {
	stat := mod.StatRegistry().GetPipelineStatistics(f.PipelineName)
	if stat == nil {
		return nil, fmt.Errorf("pipeline %s statistics not found", f.PipelineName), PipelineStatNotFoundError
	}
	names := stat.PluginIndicatorNames(f.PluginName)
	if names == nil {
		return nil, fmt.Errorf("plugin %s statistics not found", f.PluginName),
			PluginStatNotFoundError
	}

	r := new(ResultStatIndicatorNames)
	r.Names = names

	// returns with stable order
	sort.Strings(r.Names)

	return r, nil, NoneClusterError
}
func (f *FilterPluginIndicatorNames) Validate() (error, ClusterErrorType) {
	if len(f.PipelineName) == 0 {
		return fmt.Errorf("empty pipeline name in filter to retrieve " +
			"plugin statistics indicator names"), InternalServerError
	}
	if len(f.PluginName) == 0 {
		return fmt.Errorf("empty plugin name in filter to retrieve " +
			"plugin statistics indicator names"), InternalServerError
	}

	return nil, NoneClusterError
}

type FilterPluginIndicatorValue struct {
	PipelineName  string
	PluginName    string
	IndicatorName string
}

func (f *FilterPluginIndicatorValue) Execute(memberName string, mod *model.Model) (StatResult, error, ClusterErrorType) {
	stat := mod.StatRegistry().GetPipelineStatistics(f.PipelineName)
	if stat == nil {
		return nil, fmt.Errorf("pipeline %s statistics not found", f.PipelineName), PipelineStatNotFoundError
	}
	indicatorNames := stat.PluginIndicatorNames(f.PluginName)
	if indicatorNames == nil {
		return nil, fmt.Errorf("plugin %s statistics not found", f.PluginName),
			PluginStatNotFoundError
	}

	if !common.StrInSlice(f.IndicatorName, indicatorNames) {
		return nil, fmt.Errorf("indicator %s not found", f.IndicatorName),
			RetrievePluginStatIndicatorNotFoundError
	}

	r := &ResultStatIndicatorValue{
		MemberName: memberName,
		Name:       f.IndicatorName,
	}
	value, err := stat.PluginIndicatorValue(f.PluginName, f.IndicatorName)
	if err != nil {
		logger.Errorf("[retrieve the value of plugin %s statistics indicator %s in pipeline %s "+
			"from model failed: %v]", f.PluginName, f.IndicatorName,
			f.PipelineName, err)
		return nil, fmt.Errorf("evaluate indicator %s value failed", f.IndicatorName),
			RetrievePluginStatValueError
	}
	r.Value = value
	return r, nil, NoneClusterError
}
func (f *FilterPluginIndicatorValue) Validate() (error, ClusterErrorType) {
	if len(f.PipelineName) == 0 {
		return fmt.Errorf("empty pipeline name in filter to retrieve " +
			"plugin statistics indicator value"), InternalServerError
	}
	if len(f.PluginName) == 0 {
		return fmt.Errorf("empty plugin name in filter to retrieve " +
			"plugin statistics indicator value"), InternalServerError
	}
	if len(f.IndicatorName) == 0 {
		return fmt.Errorf("empty indicator name in filter to retrieve " +
			"plugin statistics indicator value"), InternalServerError
	}

	return nil, NoneClusterError
}

type FilterPluginIndicatorDesc struct {
	PipelineName  string
	PluginName    string
	IndicatorName string
}

func (f *FilterPluginIndicatorDesc) Execute(memberName string, mod *model.Model) (StatResult, error, ClusterErrorType) {
	stat := mod.StatRegistry().GetPipelineStatistics(f.PipelineName)
	if stat == nil {
		return nil, fmt.Errorf("pipeline %s statistics not found", f.PipelineName), PipelineStatNotFoundError
	}
	indicatorNames := stat.PluginIndicatorNames(f.PluginName)
	if indicatorNames == nil {
		return nil, fmt.Errorf("plugin %s statistics not found", f.PluginName),
			PluginStatNotFoundError
	}

	if !common.StrInSlice(f.IndicatorName, indicatorNames) {
		return nil, fmt.Errorf("indicator %s not found", f.IndicatorName),
			RetrievePluginStatIndicatorNotFoundError
	}

	r := new(ResultStatIndicatorDesc)
	var err error
	r.Desc, err = stat.PluginIndicatorDescription(f.PluginName, f.IndicatorName)
	if err != nil {
		logger.Errorf("[retrieve the description of plugin %s statistics indicator %s "+
			"in pipeline %s from model failed: %v]", f.PluginName, f.IndicatorName,
			f.PipelineName, err)
		return nil, fmt.Errorf("describe indicator %s failed", f.IndicatorName),
			RetrievePluginStatDescError
	}

	return r, nil, NoneClusterError
}
func (f *FilterPluginIndicatorDesc) Validate() (error, ClusterErrorType) {
	if len(f.PipelineName) == 0 {
		return fmt.Errorf("empty pipeline name in filter to retrieve " +
			"plugin statistics indicator description"), InternalServerError
	}
	if len(f.PluginName) == 0 {
		return fmt.Errorf("empty plugin name in filter to retrieve " +
			"plugin statistics indicator description"), InternalServerError
	}
	if len(f.IndicatorName) == 0 {
		return fmt.Errorf("empty indicator name in filter to retrieve " +
			"plugin statistics indicator description"), InternalServerError
	}

	return nil, NoneClusterError
}

type FilterTaskIndicatorNames struct {
	PipelineName string
}

func (f *FilterTaskIndicatorNames) Execute(memberName string, mod *model.Model) (StatResult, error, ClusterErrorType) {
	stat := mod.StatRegistry().GetPipelineStatistics(f.PipelineName)
	if stat == nil {
		return nil, fmt.Errorf("pipeline %s statistics not found", f.PipelineName), PipelineStatNotFoundError
	}
	r := new(ResultStatIndicatorNames)
	r.Names = stat.TaskIndicatorNames()

	// returns with stable order
	sort.Strings(r.Names)

	return r, nil, NoneClusterError
}
func (f *FilterTaskIndicatorNames) Validate() (error, ClusterErrorType) {
	if len(f.PipelineName) == 0 {
		return fmt.Errorf("empty pipeline name in filter to retrieve " +
			"task statistics indicator names"), InternalServerError
	}

	return nil, NoneClusterError
}

type FilterTaskIndicatorValue struct {
	PipelineName  string
	IndicatorName string
}

func (f *FilterTaskIndicatorValue) Execute(memberName string, mod *model.Model) (StatResult, error, ClusterErrorType) {
	stat := mod.StatRegistry().GetPipelineStatistics(f.PipelineName)
	if stat == nil {
		return nil, fmt.Errorf("pipeline %s statistics not found", f.PipelineName), PipelineStatNotFoundError
	}
	indicatorNames := stat.TaskIndicatorNames()
	if !common.StrInSlice(f.IndicatorName, indicatorNames) {
		return nil, fmt.Errorf("indicator %s not found", f.IndicatorName),
			RetrieveTaskStatIndicatorNotFoundError
	}

	r := &ResultStatIndicatorValue{
		MemberName: memberName,
		Name:       f.IndicatorName,
	}
	value, err := stat.TaskIndicatorValue(f.IndicatorName)
	if err != nil {
		logger.Errorf("[retrieve the value of task statistics indicator %s in pipeline %s "+
			"from model failed: %v]", f.IndicatorName, f.PipelineName, err)
		return nil, fmt.Errorf("evaluate indicator %s value failed", f.IndicatorName),
			RetrieveTaskStatValueError
	}
	r.Value = value
	return r, nil, NoneClusterError
}
func (f *FilterTaskIndicatorValue) Validate() (error, ClusterErrorType) {
	if len(f.PipelineName) == 0 {
		return fmt.Errorf("empty pipeline name in filter to retrieve " +
			"task statistics indicator value"), InternalServerError
	}
	if len(f.IndicatorName) == 0 {
		return fmt.Errorf("empty indicator name in filter to retrieve " +
			"task statistics indicator value"), InternalServerError
	}

	return nil, NoneClusterError
}

type FilterTaskIndicatorDesc struct {
	PipelineName  string
	IndicatorName string
}

func (f *FilterTaskIndicatorDesc) Execute(memberName string, mod *model.Model) (StatResult, error, ClusterErrorType) {
	stat := mod.StatRegistry().GetPipelineStatistics(f.PipelineName)
	if stat == nil {
		return nil, fmt.Errorf("pipeline %s statistics not found", f.PipelineName), PipelineStatNotFoundError
	}

	indicatorNames := stat.TaskIndicatorNames()
	if !common.StrInSlice(f.IndicatorName, indicatorNames) {
		return nil, fmt.Errorf("indicator %s not found", f.IndicatorName),
			RetrieveTaskStatIndicatorNotFoundError
	}

	r := new(ResultStatIndicatorDesc)
	var err error
	r.Desc, err = stat.TaskIndicatorDescription(f.IndicatorName)
	if err != nil {
		logger.Errorf("[retrieve the description of task statistics indicator %s in pipeline %s "+
			"from model failed: %v]", f.IndicatorName, f.PipelineName, err)
		return nil, fmt.Errorf("describe indicator %s failed", f.IndicatorName),
			RetrieveTaskStatDescError
	}

	return r, nil, NoneClusterError
}

func (f *FilterTaskIndicatorDesc) Validate() (error, ClusterErrorType) {
	if len(f.PipelineName) == 0 {
		return fmt.Errorf("empty pipeline name in filter to retrieve " +
			"task statistics indicator description"), InternalServerError
	}
	if len(f.IndicatorName) == 0 {
		return fmt.Errorf("empty indicator name in filter to retrieve " +
			"task statistics indicator description"), InternalServerError
	}

	return nil, NoneClusterError
}

// StatResult defines method by which a statistics retrieve result
// can aggregate with others
type StatResult interface {
	Aggregate(bool, map[string]StatResult) (StatResult, error)
}

type RespStat struct {
	Err     *ClusterError
	Payload StatResult
}

// RespStat implements codec.Selfer interface, which decodes or encodes itself
// These two functions are used by codec.Decoder, codec.Encoder, so
// error handling comply to codec's style
func (r *RespStat) CodecDecodeSelf(dec *codec.Decoder) {
	var typ StatType
	dec.MustDecode(&typ)
	if typ != NoneStat {
		var payload StatResult
		switch typ {
		case PipelineIndicatorNamesStat:
			fallthrough
		case PluginIndicatorNamesStat:
			fallthrough
		case TaskIndicatorNamesStat:
			payload = &ResultStatIndicatorNames{}
		case PipelineIndicatorValueStat:
			fallthrough
		case PluginIndicatorValueStat:
			fallthrough
		case TaskIndicatorValueStat:
			payload = &ResultStatIndicatorValue{}
		case PipelineIndicatorsValueStat:
			payload = &ResultStatIndicatorsValue{}
		case PipelineIndicatorDescStat:
			fallthrough
		case PluginIndicatorDescStat:
			fallthrough
		case TaskIndicatorDescStat:
			payload = &ResultStatIndicatorDesc{}
		case AggregatedIndicatorsValueStat:
			payload = &AggregatedResultStatIndicatorsValue{}
		case AggregatedIndicatorValueStat:
			payload = &AggregatedResultStatIndicatorValue{}
		default:
			panic(fmt.Sprintf("unimplemented StatType: %v", typ))
		}
		dec.MustDecode(&payload)
		r.Payload = payload
	}
	dec.MustDecode(&r.Err)
}

func (r *RespStat) CodecEncodeSelf(enc *codec.Encoder) {
	typ := NoneStat
	if r.Err == nil {
		switch v := r.Payload.(type) {
		case *ResultStatIndicatorNames:
			typ = PipelineIndicatorNamesStat
		case *ResultStatIndicatorDesc:
			typ = PipelineIndicatorDescStat
		case *ResultStatIndicatorsValue:
			typ = PipelineIndicatorsValueStat
		case *ResultStatIndicatorValue:
			typ = PipelineIndicatorValueStat
		case *AggregatedResultStatIndicatorsValue:
			typ = AggregatedIndicatorsValueStat
		case *AggregatedResultStatIndicatorValue:
			typ = AggregatedIndicatorValueStat
		default:
			panic(fmt.Sprintf("unimplemented StatType: %v", reflect.TypeOf(v)))
		}
	}
	enc.MustEncode(&typ)
	if typ != NoneStat {
		enc.MustEncode(&r.Payload)
	}
	enc.MustEncode(&r.Err)
}

type AggregatedValue struct {
	AggregatedResult interface{}            `json:"aggregated_result"`
	AggregatorName   string                 `json:"aggregator_name"`
	DetailValues     map[string]interface{} `json:"detail_values,omitempty"`
}

type AggregatedResultStatIndicatorValue struct {
	Value map[string]*AggregatedValue `json:"value"` // key is indicator name
}

func (r *AggregatedResultStatIndicatorValue) Aggregate(detail bool, others map[string]StatResult) (StatResult, error) {
	return nil, fmt.Errorf("Aggregated result type doesn't support aggregate again")
}

type AggregatedResultStatIndicatorsValue struct {
	Values map[string]AggregatedValue `json:"values"` // key is indicator name
}

func (r *AggregatedResultStatIndicatorsValue) Aggregate(detail bool, others map[string]StatResult) (StatResult, error) {
	return nil, fmt.Errorf("Aggregated result type doesn't support aggregate again")
}

type ResultStatIndicatorValue struct {
	MemberName string      `json:"-"` // omit it when writing http json result
	Name       string      `json:"name,omitempty"`
	Value      interface{} `json:"value"`
}

// Aggregate aggregate with other indicator value
func (r *ResultStatIndicatorValue) Aggregate(detail bool, otherIndicatorValue map[string]StatResult) (StatResult, error) {
	aggregatorFunc := indicatorValueAggregators[r.Name]
	if aggregatorFunc == nil && !detail {
		return nil, fmt.Errorf("can't find aggregator for indicator %s, please specify detail paramter to list indicator values", r.Name)
	}
	value := make(map[string]*AggregatedValue, 1) // only single value
	var detailValues map[string]interface{}
	if detail {
		detailValues = make(map[string]interface{}, 1+len(otherIndicatorValue))
		detailValues[r.MemberName] = r.Value
	}

	var aggregator common.StatAggregator
	if aggregatorFunc != nil {
		aggregator = aggregatorFunc()
		aggregator.Aggregate(r.Value) // aggregate self
	}
	for memberName, indicatorValue := range otherIndicatorValue {
		other, ok := indicatorValue.(*ResultStatIndicatorValue)
		if !ok {
			return nil, fmt.Errorf("must aggregate on same type: %v, but got: %v", reflect.TypeOf(r), reflect.TypeOf(indicatorValue))
		}
		if other.Name != r.Name {
			return nil, fmt.Errorf("must aggregate on same indicator name, this: %s, other is: %s", r.Name, other.Name)
		}
		if aggregator != nil {
			aggregator.Aggregate(other.Value)
		}
		if detail {
			detailValues[memberName] = other.Value
		}
	}

	v := &AggregatedValue{
		DetailValues: detailValues,
	}
	if aggregator != nil {
		v.AggregatedResult = aggregator.Result()
		v.AggregatorName = aggregator.String()
	}
	value[r.Name] = v
	return &AggregatedResultStatIndicatorValue{value}, nil
}

type ResultStatIndicatorsValue struct {
	MemberName string                 `json:"member_name,omitempty"`
	Values     map[string]interface{} `json:"values,omitempty"` // key is indicator name
}

// Aggregate aggregate with other indicators value
func (r *ResultStatIndicatorsValue) Aggregate(detail bool, otherIndicatorsValue map[string]StatResult) (StatResult, error) {
	values := make(map[string]AggregatedValue, len(r.Values))
	for key, value := range r.Values {
		if value == nil { // this indicator failed, so skip it
			continue
		}
		var detailValues map[string]interface{}
		if detail {
			detailValues = make(map[string]interface{}, 1+len(otherIndicatorsValue))
			detailValues[r.MemberName] = value
		}
		aggregatorFunc := indicatorValueAggregators[key]
		if aggregatorFunc == nil && !detail {
			return nil, fmt.Errorf("can't find aggregator for indicator %s, please specify detail paramter to list indicator values", key)
		}

		var aggregator common.StatAggregator
		if aggregatorFunc != nil {
			aggregator = aggregatorFunc()
			aggregator.Aggregate(value) // ignore error safely
		}

		for memberName, v := range otherIndicatorsValue {
			indicatorsValue, ok := v.(*ResultStatIndicatorsValue)
			if !ok {
				return nil, fmt.Errorf("must aggregate on same type: %v, but got: %v", reflect.TypeOf(r), reflect.TypeOf(v))
			}
			if aggregator != nil {
				aggregator.Aggregate(indicatorsValue.Values[key])
			}
			if detail {
				detailValues[memberName] = indicatorsValue.Values[key]
			}
		}

		r := AggregatedValue{
			DetailValues: detailValues,
		}
		if aggregator != nil {
			r.AggregatedResult = aggregator.Result()
			r.AggregatorName = aggregator.String()
		}
		values[key] = r
	}
	return &AggregatedResultStatIndicatorsValue{values}, nil
}

type ResultStatIndicatorDesc struct {
	Desc string `json:"desc"`
}

// Aggregate aggregate with other desc
// We don't need to aggregate other indicator descs,
// because we already has it's description
func (r *ResultStatIndicatorDesc) Aggregate(detail bool, otherDescs map[string]StatResult) (StatResult, error) {
	return r, nil
}

type ResultStatIndicatorNames struct {
	Names []string `json:"names"`
}

// Aggregate aggregate with other names
func (r *ResultStatIndicatorNames) Aggregate(detail bool, indicatorNames map[string]StatResult) (StatResult, error) {
	memory := make(map[string]struct{})
	for _, name := range r.Names {
		memory[name] = struct{}{}
	}
	for _, indicatorName := range indicatorNames {
		other, ok := indicatorName.(*ResultStatIndicatorNames)
		if !ok {
			return nil, fmt.Errorf("must aggregate on same type: %v, but got: %v", reflect.TypeOf(r), reflect.TypeOf(indicatorName))
		}
		for _, name := range other.Names {
			memory[name] = struct{}{}
		}
	}
	names := make([]string, 0, len(memory))
	for name := range memory {
		names = append(names, name)
	}

	// returns with stable order
	sort.Strings(names)

	return &ResultStatIndicatorNames{names}, nil
}

// TODO(shengdong) below definitions can be moved to gateway-types. They are both used
// in easegateway-client and gateway restful API server
type ClusterResp struct {
	Time time.Time `json:"time" description:"Timestamp of the response"`
}

//
// Cluster health messages
//

type GroupStatus string

const (
	Green  GroupStatus = GroupStatus("green")
	Yellow GroupStatus = GroupStatus("yellow")
	Red    GroupStatus = GroupStatus("red")
)

func (s GroupStatus) String() string {
	return string(s)
}

// Cluster infomation messages

type OpLogInfo struct {
	OpLogInnerInfo
	OpLogStatus
}

type OpLogGroupInfo struct {
	OpLogStatus
	UnSyncedMembersCount uint64   `json:"unsynced_members_count" description:"The count of unsynced members"`
	UnSyncedMembers      []string `json:"unsynced_members" description:"The members names on which operation log is unsynchronized"`
}

type OpLogStatus struct {
	Synced      bool  `json:"synced" description:"Whether operation log is synced with writer node. For a normal group/node, this value should be true." default:"false"`
	MaxSequence int64 `json:"max_sequence" description:"Operation log maximum sequence. Negative value means invalid max sequence." default:"-1"`
	MinSequence int64 `json:"min_sequence" description:"Operation log minimum sequence. Negative value means invalid max sequence." default:"-1"`
}

type OpLogInnerInfo struct {
	DataPath     string `json:"data_path" description:"Operation log local data path on the node."`
	SizeBytes    uint64 `json:"size_bytes" description:"Operation log files size in bytes"`
	SyncProgress int8   `json:"sync_progress" description:"Operation log syncing progress, 100 indicates the operation log is fully synchronized. Negative value means invalid sync progress." default:"-1" minimum:"0" maximum:"100"`
	SyncLag      int64  `json:"sync_lag" description:"Indicate the operation log syncing lag. This is the difference between writer node's operation log max sequence and specific member's max sequence. In normal case, this value should be zero. Negative value means invalid sync lag." default:"-1"`
}

type MemberInfo struct {
	Name     string `json:"name" description:"Member name"`
	Endpoint string `json:"endpoint" description:"Member endpoint"`
	Mode     string `json:"mode" enum:"write|read" description:"Member mode"`
}

type MembersInfo struct {
	FailedMembersCount uint64       `json:"failed_members_count" description:"Count of failed members"`
	FailedMembers      []MemberInfo `json:"failed_members" description:"The failed(can't connect these members) members"`
	AliveMembersCount  uint64       `json:"alive_members_count" description:"Count of alive members"`
	AliveMembers       []MemberInfo `json:"alive_members" description:"The alive members"`
}

type GossipConfig struct {
	GossipPacketSizeBytes  int           `json:"gossip_packet_size_bytes" description:"Maximum number of bytes that memberlist will put in a packet (this will be for UDP packets by default with a NetTransport)."`
	GossipIntervalMillisec time.Duration `json:"gossip_interval_ms" type:"integer" description:"Interval in millisecond between sending messages that need to be gossiped that haven't been able to piggyback on probing messages."`
}

type OpLogConfig struct {
	MaxSeqGapToPull     uint16        `json:"max_seq_gap_to_pull" description:"Max gap of sequence of operation logs deciding whether to wait for missing operations or not"`
	PullMaxCountOnce    uint16        `json:"pull_max_count_once" description:"Max count of pulling operation logs once"`
	PullIntervalSeconds time.Duration `json:"pull_interval_seconds" type:"integer" description:"Interval of pulling operation logs in second"`
	PullTimeoutSeconds  time.Duration `json:"pull_timeout_seconds" type:"integer" description:"Timeout of pulling operation logs in second"`
}

type memberConfig struct {
	ClusterDefaultOpTimeoutSec time.Duration `json:"cluster_default_op_timeout_sec" type:"integer" description:"default timeout of cluster operation in second"`
	GossipConfig               GossipConfig  `json:"gossip" description:"Gossip configurations"`
	OpLogConfig                OpLogConfig   `json:"oplog" description:"Operation log configurations"`
}

type MemberInnerInfo struct {
	Config    memberConfig `json:"config" description:"Member configurations"`
	OpLogInfo OpLogInfo    `json:"oplog" description:"Member's operation log information"`
}

type RespQueryGroupPayload struct {
	ClusterResp
	GroupName      string         `json:"group_name" description:"Group name"`
	OpLogGroupInfo OpLogGroupInfo `json:"oplog" description:"Group's operation log information"`
	MembersInfo
	TimeoutNodes []string `json:"timeout_nodes,omitempty" description:"Indicates timeout nodes if partially success"`
}

type RespQueryGroupHealthPayload struct {
	ClusterResp
	Status       GroupStatus `json:"status" description:"Indicates the group health status." enum:"green|yellow|red"`
	Description  string      `json:"description" description:"Indicates description of the status. For a green status, this value will be empty"`
	TimeoutNodes []string    `json:"timeout_nodes,omitempty" description:"Indicates timeout nodes if partially success"`
}

// ReqQueryMember | ReqQueryMemberslist | ReqQueryGroup
// Pack Header: queryMemberMessage | queryMemberListMessage | queryGroupMessage
type (
	ReqQueryMember  struct{}
	RespQueryMember struct {
		ClusterResp
		MemberInnerInfo
	}

	ReqQueryMembersList  struct{}
	RespQueryMembersList struct {
		ClusterResp
		MembersInfo
	}

	ReqQueryGroup  struct{}
	RespQueryGroup struct {
		Err *ClusterError
		RespQueryGroupPayload
	}

	ReqQueryGroupHealth  struct{}
	RespQueryGroupHealth struct {
		Err *ClusterError
		RespQueryGroupHealthPayload
	}
)
