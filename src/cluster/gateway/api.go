package gateway

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"cluster"
	"logger"
	"option"
)

// meta
func (gc *GatewayCluster) RetrieveGroups() []string {
	groups := make([]string, 0)

	if gc.Stopped() {
		return groups
	}

	totalMembers := gc.cluster.Members()
	groupsBook := make(map[string]struct{})

	for _, member := range totalMembers {
		group := member.NodeTags[groupTagKey]
		if _, ok := groupsBook[group]; ok {
			continue
		}
		groupsBook[group] = struct{}{}

		groups = append(groups, group)
	}

	sort.Strings(groups)
	return groups
}

type GroupInfo struct {
	Name        string   `json:"group_name"`
	GroupMaxSeq string   `json:"group_operation_sequence"`
	WriteNode   string   `json:"write_node"`
	ReadNodes   []string `json:"read_nodes"`
}

func (gc *GatewayCluster) RetrieveGroup(group string) *GroupInfo {
	if gc.Stopped() {
		return nil
	}

	totalMembers := gc.cluster.Members()
	groupInfo := &GroupInfo{
		Name: group,
	}
	groupInfo.ReadNodes = make([]string, 0)

	for _, member := range totalMembers {
		if member.NodeTags[groupTagKey] != groupInfo.Name {
			continue
		}

		if member.NodeTags[modeTagKey] == string(WriteMode) {
			groupInfo.WriteNode = member.NodeName
		} else {
			groupInfo.ReadNodes = append(groupInfo.ReadNodes, member.NodeName)
		}
	}

	groupMaxSeqStr := "UNKNOW"
	groupMaxSeq, err := gc.QueryGroupMaxSeq(option.ClusterGroup, 10*time.Second)
	if err == nil {
		groupMaxSeqStr = fmt.Sprintf("%d", groupMaxSeq)
	}
	groupInfo.GroupMaxSeq = groupMaxSeqStr

	return groupInfo
}

func (gc *GatewayCluster) RetrieveMembers() []string {
	members := make([]string, 0)

	if gc.Stopped() {
		return members
	}

	totalMembers := gc.cluster.Members()

	for _, member := range totalMembers {
		members = append(members, formatMember(&member))
	}

	sort.Strings(members)
	return members
}

type MemberInfo struct {
	Name                  string   `json:"node_name"`
	Mode                  string   `json:"node_mode"`
	Group                 string   `json:"group_name"`
	GroupMaxSeq           string   `json:"group_operation_sequence"`
	LocalMaxSeq           string   `json:"local_operation_sequence"`
	Peers                 []string `json:"alive_peers_in_group"`
	OPLogMaxSeqGapToPull  uint16   `json:"oplog_max_seq_gap_to_pull"`
	OPLogPullMaxCountOnce uint16   `json:"oplog_pull_max_count_once"`
	OPLogPullInterval     int      `json:"oplog_pull_interval_in_second"`
	OPLogPullTimeout      int      `json:"oplog_pull_timeout_in_second"`
}

func formatMember(member *cluster.Member) string {
	if member == nil {
		return ""
	}
	return fmt.Sprintf("%s (%s:%d) (%s:%s)",
		member.NodeName, member.Address.String(), member.Port,
		member.NodeTags[groupTagKey], member.NodeTags[modeTagKey])
}

func (gc *GatewayCluster) RetrieveMember(nodeName string) *MemberInfo {
	if gc.Stopped() {
		return nil
	}

	groupMaxSeqStr := "UNKNOW"
	groupMaxSeq, err := gc.QueryGroupMaxSeq(option.ClusterGroup, 10*time.Second)
	if err == nil {
		groupMaxSeqStr = fmt.Sprintf("%d", groupMaxSeq)
	}

	// keep same datatype of group max sequence for client
	localMaxSeqStr := fmt.Sprintf("%d", gc.OPLog().MaxSeq())

	peers := make([]string, 0)
	for _, member := range gc.RestAliveMembersInSameGroup() {
		peers = append(peers, formatMember(&member))
	}

	return &MemberInfo{
		Name:                  gc.NodeName(),
		Mode:                  strings.ToLower(gc.Mode().String()),
		Group:                 option.ClusterGroup,
		GroupMaxSeq:           groupMaxSeqStr,
		LocalMaxSeq:           localMaxSeqStr,
		Peers:                 peers,
		OPLogMaxSeqGapToPull:  option.OPLogMaxSeqGapToPull,
		OPLogPullMaxCountOnce: option.OPLogPullMaxCountOnce,
		OPLogPullInterval:     int(option.OPLogPullInterval.Seconds()),
		OPLogPullTimeout:      int(option.OPLogPullTimeout.Seconds()),
	}
}

// operation
func (gc *GatewayCluster) QueryGroupMaxSeq(group string, timeout time.Duration) (uint64, *ClusterError) {
	if gc.Stopped() {
		return 0, newClusterError("can not query max operation sequence due to cluster gone",
			IssueMemberGoneError)
	}

	req := new(ReqQueryGroupMaxSeq)
	requestPayload, err := cluster.PackWithHeader(req, uint8(queryGroupMaxSeqMessage))
	if err != nil {
		logger.Errorf("[BUG: pack request (header=%d) to %#v failed: %v]",
			uint8(queryGroupMaxSeqMessage), req, err)

		return 0, newClusterError(
			fmt.Sprintf("pack request (header=%d) to %#v failed: %v",
				uint8(queryGroupMaxSeqMessage), req, err),
			InternalServerError)
	}

	requestParam := cluster.RequestParam{
		TargetNodeTags: map[string]string{
			groupTagKey: group,
			modeTagKey:  WriteMode.String(),
		},
		Timeout:            timeout,
		ResponseRelayCount: 1, // fault tolerance on network issue
	}

	requestName := fmt.Sprintf("(group:%s)query_group_max_sequence", group)

	logger.Debugf("issue querySequence: requestName(%s) timeout(%s)",
		requestName, timeout)

	future, err := gc.cluster.Request(requestName, requestPayload, &requestParam)
	if err != nil {
		return 0, newClusterError(fmt.Sprintf("query max sequence failed: %v", err), InternalServerError)
	}

	var memberResp *cluster.MemberResponse

	select {
	case r, ok := <-future.Response():
		if !ok {
			return 0, newClusterError("query max sequence in the group timeout", TimeoutError)
		}
		memberResp = r
	case <-gc.stopChan:
		return 0, newClusterError("the member gone during issuing max sequence query", IssueMemberGoneError)
	}

	if len(memberResp.Payload) == 0 {
		return 0, newClusterError("query max sequence responds empty response", InternalServerError)
	}

	var resp RespQueryGroupMaxSeq
	err = cluster.Unpack(memberResp.Payload[1:], &resp)
	if err != nil {
		return 0, newClusterError(fmt.Sprintf("unpack max sequence response failed: %v", err),
			InternalServerError)
	}

	return uint64(resp), nil
}

func (gc *GatewayCluster) CreatePlugin(group string, timeout time.Duration, startSeq uint64, syncAll bool,
	typ string, conf []byte) *ClusterError {

	if gc.Stopped() {
		return newClusterError(
			fmt.Sprintf("can not create plugin (sequence=%d) due to cluster gone", startSeq),
			IssueMemberGoneError)
	}

	operation := Operation{
		ContentCreatePlugin: &ContentCreatePlugin{
			Type:   typ,
			Config: conf,
		},
	}

	requestName := fmt.Sprintf("(group:%s)create_plugin", group)

	return gc.issueOperation(group, timeout, requestName, startSeq, syncAll, &operation)
}

func (gc *GatewayCluster) UpdatePlugin(group string, timeout time.Duration, startSeq uint64, syncAll bool,
	typ string, conf []byte) *ClusterError {

	if gc.Stopped() {
		return newClusterError(
			fmt.Sprintf("can not update plugin (sequence=%d) due to cluster gone", startSeq),
			IssueMemberGoneError)
	}

	operation := Operation{
		ContentUpdatePlugin: &ContentUpdatePlugin{
			Type:   typ,
			Config: conf,
		},
	}

	requestName := fmt.Sprintf("(group:%s)update_plugin", group)

	return gc.issueOperation(group, timeout, requestName, startSeq, syncAll, &operation)
}

func (gc *GatewayCluster) DeletePlugin(group string, timeout time.Duration, startSeq uint64, syncAll bool,
	name string) *ClusterError {

	if gc.Stopped() {
		return newClusterError(
			fmt.Sprintf("can not delete plugin (sequence=%d) due to cluster gone", startSeq),
			IssueMemberGoneError)
	}

	operation := Operation{
		ContentDeletePlugin: &ContentDeletePlugin{
			Name: name,
		},
	}

	requestName := fmt.Sprintf("(group:%s)delete_plugin", group)

	return gc.issueOperation(group, timeout, requestName, startSeq, syncAll, &operation)
}

func (gc *GatewayCluster) CreatePipeline(group string, timeout time.Duration, startSeq uint64, syncAll bool,
	typ string, conf []byte) *ClusterError {

	if gc.Stopped() {
		return newClusterError(
			fmt.Sprintf("can not create pipeline (sequence=%d) due to cluster gone", startSeq),
			IssueMemberGoneError)
	}

	operation := Operation{
		ContentCreatePipeline: &ContentCreatePipeline{
			Type:   typ,
			Config: conf,
		},
	}

	requestName := fmt.Sprintf("(group:%s)create_pipeline", group)

	return gc.issueOperation(group, timeout, requestName, startSeq, syncAll, &operation)
}

func (gc *GatewayCluster) UpdatePipeline(group string, timeout time.Duration, startSeq uint64, syncAll bool,
	typ string, conf []byte) *ClusterError {

	if gc.Stopped() {
		return newClusterError(
			fmt.Sprintf("can not update pipeline (sequence=%d) due to cluster gone", startSeq),
			IssueMemberGoneError)
	}

	operation := Operation{
		ContentUpdatePipeline: &ContentUpdatePipeline{
			Type:   typ,
			Config: conf,
		},
	}

	requestName := fmt.Sprintf("(group:%s)update_pipeline", group)

	return gc.issueOperation(group, timeout, requestName, startSeq, syncAll, &operation)
}

func (gc *GatewayCluster) DeletePipeline(group string, timeout time.Duration, startSeq uint64, syncAll bool,
	name string) *ClusterError {

	if gc.Stopped() {
		return newClusterError(
			fmt.Sprintf("can not delete pipeline (sequence=%d) due to cluster gone", startSeq),
			IssueMemberGoneError)
	}

	operation := Operation{
		ContentDeletePipeline: &ContentDeletePipeline{
			Name: name,
		},
	}

	requestName := fmt.Sprintf("(group:%s)delete_pipeline", group)

	return gc.issueOperation(group, timeout, requestName, startSeq, syncAll, &operation)
}

// retrieve
func (gc *GatewayCluster) RetrievePlugin(group string, timeout time.Duration, syncAll bool,
	name string) (*ResultRetrievePlugin, *ClusterError) {

	if gc.Stopped() {
		return nil, newClusterError("can not retrieve plugin due to cluster gone", IssueMemberGoneError)
	}

	filter := FilterRetrievePlugin{
		Name: name,
	}

	requestName := fmt.Sprintf("(group:%s)retrive_plugin", group)

	resp, err := gc.issueRetrieve(group, timeout, requestName, syncAll, &filter)
	if err != nil {
		return nil, err
	}

	ret, ok := resp.(*ResultRetrievePlugin)
	if !ok {
		logger.Errorf("[BUG: retrieve plugin returns invalid result, got type %T]", resp)
		return nil, newClusterError("retrieve plugin returns invalid result", InternalServerError)
	}

	return ret, nil
}

func (gc *GatewayCluster) RetrievePlugins(group string, timeout time.Duration, syncAll bool,
	namePattern string, types []string) (*ResultRetrievePlugins, *ClusterError) {

	if gc.Stopped() {
		return nil, newClusterError("can not retrieve plugins due to cluster gone", IssueMemberGoneError)
	}

	filter := FilterRetrievePlugins{
		NamePattern: namePattern,
		Types:       types,
	}

	requestName := fmt.Sprintf("(group:%s)retrive_plugins", group)

	resp, err := gc.issueRetrieve(group, timeout, requestName, syncAll, &filter)
	if err != nil {
		return nil, err
	}

	ret, ok := resp.(*ResultRetrievePlugins)
	if !ok {
		logger.Errorf("[BUG: retrieve plugins returns invalid result, got type %T]", resp)
		return nil, newClusterError("retrieve plugins returns invalid result", InternalServerError)
	}

	return ret, nil
}

func (gc *GatewayCluster) RetrievePipeline(group string, timeout time.Duration, syncAll bool,
	name string) (*ResultRetrievePipeline, *ClusterError) {

	if gc.Stopped() {
		return nil, newClusterError("can not retrieve pipeline due to cluster gone", IssueMemberGoneError)
	}

	filter := FilterRetrievePipeline{
		Name: name,
	}

	requestName := fmt.Sprintf("(group:%s)retrive_pipeline", group)

	resp, err := gc.issueRetrieve(group, timeout, requestName, syncAll, &filter)
	if err != nil {
		return nil, err
	}

	ret, ok := resp.(*ResultRetrievePipeline)
	if !ok {
		logger.Errorf("[BUG: retrieve pipeline returns invalid result, got type %T]", resp)
		return nil, newClusterError("retrieve pipeline returns invalid result", InternalServerError)
	}

	return ret, nil
}

func (gc *GatewayCluster) RetrievePipelines(group string, timeout time.Duration, syncAll bool,
	namePattern string, types []string) (*ResultRetrievePipelines, *ClusterError) {

	if gc.Stopped() {
		return nil, newClusterError("can not retrieve pipelines due to cluster gone", IssueMemberGoneError)
	}

	filter := FilterRetrievePipelines{
		NamePattern: namePattern,
		Types:       types,
	}

	requestName := fmt.Sprintf("(group:%s)retrive_pipelines", group)

	resp, err := gc.issueRetrieve(group, timeout, requestName, syncAll, &filter)
	if err != nil {
		return nil, err
	}

	ret, ok := resp.(*ResultRetrievePipelines)
	if !ok {
		logger.Errorf("[BUG: retrieve pipelines returns invalid result, got type %T]", resp)
		return nil, newClusterError("retrieve pipelines returns invalid result", InternalServerError)
	}

	return ret, nil
}

func (gc *GatewayCluster) RetrievePluginTypes(group string, timeout time.Duration,
	syncAll bool) (*ResultRetrievePluginTypes, *ClusterError) {

	if gc.Stopped() {
		return nil, newClusterError("can not retrieve plugin types due to cluster gone", IssueMemberGoneError)
	}

	requestName := fmt.Sprintf("(group:%s)retrive_plugin_types", group)

	resp, err := gc.issueRetrieve(group, timeout, requestName, syncAll, &FilterRetrievePluginTypes{})
	if err != nil {
		return nil, err
	}

	ret, ok := resp.(*ResultRetrievePluginTypes)
	if !ok {
		logger.Errorf("[BUG: retrieve plugin types returns invalid result, got type %T]", resp)
		return nil, newClusterError("retrieve plugin types returns invalid result", InternalServerError)
	}

	return ret, nil
}

func (gc *GatewayCluster) RetrievePipelineTypes(group string, timeout time.Duration,
	syncAll bool) (*ResultRetrievePipelineTypes, *ClusterError) {

	if gc.Stopped() {
		return nil, newClusterError("can not retrieve pipeline types due to cluster gone",
			IssueMemberGoneError)
	}

	requestName := fmt.Sprintf("(group:%s)retrive_pipeline_types", group)

	resp, err := gc.issueRetrieve(group, timeout, requestName, syncAll, &FilterRetrievePluginTypes{})
	if err != nil {
		return nil, err
	}

	ret, ok := resp.(*ResultRetrievePipelineTypes)
	if !ok {
		logger.Errorf("[BUG: retrieve pipeline types returns invalid result, got type %T]", resp)
		return nil, newClusterError("retrieve pipeline types returns invalid result", InternalServerError)
	}

	return ret, nil
}

// statistics
func (gc *GatewayCluster) StatPipelineIndicatorNames(group string, timeout time.Duration,
	pipelineName string) (*ResultStatIndicatorNames, *ClusterError) {

	if gc.Stopped() {
		return nil, newClusterError("can not retrieve pipeline statistics indicator names due to cluster gone",
			IssueMemberGoneError)
	}

	filter := FilterPipelineIndicatorNames{
		PipelineName: pipelineName,
	}

	requestName := fmt.Sprintf("(group(%s)stat_pipleine_indicator_names)", group)

	resp, err := gc.issueStat(group, timeout, requestName, &filter)
	if err != nil {
		return nil, err
	}

	ret, ok := resp.(*ResultStatIndicatorNames)
	if !ok {
		logger.Errorf("[BUG: stat pipeline indicator names returns invalid result, got type %T]", resp)
		return nil, newClusterError("stat pipeline indicator names returns invalid result",
			InternalServerError)
	}

	return ret, nil
}

func (gc *GatewayCluster) StatPipelineIndicatorValue(group string, timeout time.Duration,
	pipelineName, indicatorName string) (*ResultStatIndicatorValue, *ClusterError) {

	if gc.Stopped() {
		return nil, newClusterError("can not retrieve pipeline statistics indicator value due to cluster gone",
			IssueMemberGoneError)
	}

	filter := FilterPipelineIndicatorValue{
		PipelineName:  pipelineName,
		IndicatorName: indicatorName,
	}

	requestName := fmt.Sprintf("(group(%s)stat_pipleine_indicator_value)", group)

	resp, err := gc.issueStat(group, timeout, requestName, &filter)
	if err != nil {
		return nil, err
	}

	ret, ok := resp.(*ResultStatIndicatorValue)
	if !ok {
		logger.Errorf("[BUG: stat pipeline indicator value returns invalid result, got type %T]", resp)
		return nil, newClusterError("stat pipeline indicator value returns invalid result", InternalServerError)
	}

	return ret, nil
}

func (gc *GatewayCluster) StatPipelineIndicatorDesc(group string, timeout time.Duration,
	pipelineName, indicatorName string) (*ResultStatIndicatorDesc, *ClusterError) {

	if gc.Stopped() {
		return nil, newClusterError(
			"can not retrieve pipeline statistics indicator description due to cluster gone",
			IssueMemberGoneError)
	}

	filter := FilterPipelineIndicatorDesc{
		PipelineName:  pipelineName,
		IndicatorName: indicatorName,
	}

	requestName := fmt.Sprintf("(group(%s)stat_pipleine_indicator_desc)", group)

	resp, err := gc.issueStat(group, timeout, requestName, &filter)
	if err != nil {
		return nil, err
	}

	ret, ok := resp.(*ResultStatIndicatorDesc)
	if !ok {
		logger.Errorf("[BUG: stat pipeline indicator description returns invalid result, got type %T]", resp)
		return nil, newClusterError("stat pipeline indicator description returns invalid result",
			InternalServerError)
	}

	return ret, nil
}

func (gc *GatewayCluster) StatPluginIndicatorNames(group string, timeout time.Duration,
	pipelineName, pluginName string) (*ResultStatIndicatorNames, *ClusterError) {

	if gc.Stopped() {
		return nil, newClusterError("can not retrieve plugin statistics indicator names due to cluster gone",
			IssueMemberGoneError)
	}

	filter := FilterPluginIndicatorNames{
		PipelineName: pipelineName,
		PluginName:   pluginName,
	}

	requestName := fmt.Sprintf("(group(%s)stat_plugin_indicator_names)", group)

	resp, err := gc.issueStat(group, timeout, requestName, &filter)
	if err != nil {
		return nil, err
	}

	ret, ok := resp.(*ResultStatIndicatorNames)
	if !ok {
		logger.Errorf("[BUG: stat plugin indicator names returns invalid result, got type %T]", resp)
		return nil, newClusterError("stat plugin indicator names returns invalid result", InternalServerError)
	}

	return ret, nil
}

func (gc *GatewayCluster) StatPluginIndicatorValue(group string, timeout time.Duration,
	pipelineName, pluginName, indicatorName string) (*ResultStatIndicatorValue, *ClusterError) {

	if gc.Stopped() {
		return nil, newClusterError("can not retrieve plugin statistics indicator value due to cluster gone",
			IssueMemberGoneError)
	}

	filter := FilterPluginIndicatorValue{
		PipelineName:  pipelineName,
		PluginName:    pluginName,
		IndicatorName: indicatorName,
	}

	requestName := fmt.Sprintf("(group(%s)stat_plugin_indicator_value)", group)

	resp, err := gc.issueStat(group, timeout, requestName, &filter)
	if err != nil {
		return nil, err
	}

	ret, ok := resp.(*ResultStatIndicatorValue)
	if !ok {
		logger.Errorf("[BUG: stat plugin indicator value returns invalid result, got type %T]", resp)
		return nil, newClusterError("stat plugin indicator value returns invalid result", InternalServerError)
	}

	return ret, nil
}

func (gc *GatewayCluster) StatPluginIndicatorDesc(group string, timeout time.Duration,
	pipelineName, pluginName, indicatorName string) (*ResultStatIndicatorDesc, *ClusterError) {

	if gc.Stopped() {
		return nil, newClusterError(
			"can not retrieve plugin statistics indicator description due to cluster gone",
			IssueMemberGoneError)
	}

	filter := FilterPluginIndicatorDesc{
		PipelineName:  pipelineName,
		PluginName:    pluginName,
		IndicatorName: indicatorName,
	}

	requestName := fmt.Sprintf("(group(%s)stat_plugin_indicator_desc)", group)

	resp, err := gc.issueStat(group, timeout, requestName, &filter)
	if err != nil {
		return nil, err
	}

	ret, ok := resp.(*ResultStatIndicatorDesc)
	if !ok {
		logger.Errorf("[BUG: stat plugin indicator description returns invalid result, got type %T]", resp)
		return nil, newClusterError("stat plugin indicator description returns invalid result",
			InternalServerError)
	}

	return ret, nil
}

func (gc *GatewayCluster) StatTaskIndicatorNames(group string, timeout time.Duration,
	pipelineName string) (*ResultStatIndicatorNames, *ClusterError) {

	if gc.Stopped() {
		return nil, newClusterError("can not retrieve task statistics indicator names due to cluster gone",
			IssueMemberGoneError)
	}

	filter := FilterTaskIndicatorNames{
		PipelineName: pipelineName,
	}

	requestName := fmt.Sprintf("(group(%s)stat_task_indicator_names)", group)

	resp, err := gc.issueStat(group, timeout, requestName, &filter)
	if err != nil {
		return nil, err
	}

	ret, ok := resp.(*ResultStatIndicatorNames)
	if !ok {
		logger.Errorf("[BUG: stat task indicator names returns invalid result, got type %T]", resp)
		return nil, newClusterError("stat task indicator names returns invalid result", InternalServerError)
	}

	return ret, nil
}

func (gc *GatewayCluster) StatTaskIndicatorValue(group string, timeout time.Duration,
	pipelineName, indicatorName string) (*ResultStatIndicatorValue, *ClusterError) {

	if gc.Stopped() {
		return nil, newClusterError("can not retrieve task statistics indicator value due to cluster gone",
			IssueMemberGoneError)
	}

	filter := FilterTaskIndicatorValue{
		PipelineName:  pipelineName,
		IndicatorName: indicatorName,
	}

	requestName := fmt.Sprintf("(group(%s)stat_task_indicator_value)", group)

	resp, err := gc.issueStat(group, timeout, requestName, &filter)
	if err != nil {
		return nil, err
	}

	ret, ok := resp.(*ResultStatIndicatorValue)
	if !ok {
		logger.Errorf("[BUG: stat task indicator value returns invalid result, got type %T]", resp)
		return nil, newClusterError("stat task indicator value returns invalid result", InternalServerError)
	}

	return ret, nil
}

func (gc *GatewayCluster) StatTaskIndicatorDesc(group string, timeout time.Duration,
	pipelineName, indicatorName string) (*ResultStatIndicatorDesc, *ClusterError) {

	if gc.Stopped() {
		return nil, newClusterError(
			"can not retrieve task statistics indicator description due to cluster gone",
			IssueMemberGoneError)
	}

	filter := FilterTaskIndicatorDesc{
		PipelineName:  pipelineName,
		IndicatorName: indicatorName,
	}

	requestName := fmt.Sprintf("(group(%s)stat_task_indicator_desc)", group)

	resp, err := gc.issueStat(group, timeout, requestName, &filter)
	if err != nil {
		return nil, err
	}

	ret, ok := resp.(*ResultStatIndicatorDesc)
	if !ok {
		logger.Errorf("[BUG: stat task indicator description returns invalid result, got type %T]", resp)
		return nil, newClusterError("stat task indicator description returns invalid result",
			InternalServerError)
	}

	return ret, nil
}
