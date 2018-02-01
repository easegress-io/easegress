package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"config"
	"model"
	"plugins"
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

type (
	MessageType uint8
)

// querySeqMessage

type (
	// Pack Header: querySeqMessage
	ReqQuerySeq struct{}
	// Pack Header: querySeqMessage
	RespQuerySeq struct {
		Min uint64
		Max uint64
	}
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
		Detail bool // It is Leveraged by value[s] filters.

		FilterPipelineIndicatorNames  *FilterPipelineIndicatorNames
		FilterPipelineIndicatorValue  *FilterPipelineIndicatorValue
		FilterPipelineIndicatorsValue *FilterPipelineIndicatorsValue
		FilterPipelineIndicatorDesc   *FilterPipelineIndicatorDesc
		FilterPluginIndicatorNames    *FilterPluginIndicatorNames
		FilterPluginIndicatorValue    *FilterPluginIndicatorValue
		FilterPluginIndicatorDesc     *FilterPluginIndicatorDesc
		FilterTaskIndicatorNames      *FilterTaskIndicatorNames
		FilterTaskIndicatorValue      *FilterTaskIndicatorValue
		FilterTaskIndicatorDesc       *FilterTaskIndicatorDesc
	}
	// Pack Header: statMessage | statRelayMessage
	RespStat struct {
		Err *ClusterError

		Names  []byte // json
		Value  []byte // json
		Values []byte // json
		Desc   []byte // json
	}
	FilterPipelineIndicatorNames struct {
		PipelineName string
	}
	FilterPipelineIndicatorValue struct {
		PipelineName  string
		IndicatorName string
	}
	FilterPipelineIndicatorsValue struct {
		PipelineName   string
		IndicatorNames []string
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
	ResultStatIndicatorsValue struct {
		Values map[string]interface{} `json:"values"`
	}
	ResultStatIndicatorDesc struct {
		Desc interface{} `json:"desc"`
	}
	resultStatAggregateValue struct {
		AggregatedResult interface{}            `json:"aggregated_result"`
		AggregatorName   string                 `json:"aggregator_name"`
		DetailValues     map[string]interface{} `json:"detail_values,omitempty"`
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
	OpLogGroupInfo OpLogGroupInfo `json:"oplog" description:"Group's operation log information"`
	MembersInfo
}

type RespQueryGroupHealthPayload struct {
	ClusterResp
	Status      GroupStatus `json:"status" description:"Indicates the group health status." enum:"green|yellow|red"`
	Description string      `json:"description" description:"Indicates description of the status. For a green status, this value will be empty"`
}

type (
	// Pack Header: queryMemberMessage
	ReqQueryMember struct{}
	// Pack Header: queryMemberMessage
	RespQueryMember struct {
		ClusterResp
		MemberInnerInfo
	}

	// Pack Header: queryMemberListMessage
	ReqQueryMembersList struct{}
	// Pack Header: queryMemberListMessage
	RespQueryMembersList struct {
		ClusterResp
		MembersInfo
	}

	// Pack Header: queryMemberListMessage
	ReqQueryGroup struct{}
	// Pack Header: queryMemberListMessage
	RespQueryGroup struct {
		Err *ClusterError
		RespQueryGroupPayload
	}

	// Pack Header: queryGroupHealth
	ReqQueryGroupHealth struct{}
	// Pack Header: queryMemberListMessage
	RespQueryGroupHealth struct {
		Err *ClusterError
		RespQueryGroupHealthPayload
	}
)
