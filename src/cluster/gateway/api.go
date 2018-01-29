package gateway

import (
	"fmt"
	"strings"
	"time"

	"cluster"
	"common"
	"logger"
)

// cluster infos

// RetrieveGroupsList retrieve groups list from current node
func (gc *GatewayCluster) RetrieveGroupsList() ([]string, *ClusterError) {
	if gc.Stopped() {
		return nil, newClusterError("can not query group info due to cluster gone",
			IssueMemberGoneError)
	}

	groups := gc.groupsInCluster()

	return groups, nil
}

// RetrieveGroups retrieve all groups detail information from corresponding writer nodes
func (gc *GatewayCluster) RetrieveGroups(timeout time.Duration) (map[string]*RespQueryGroup, *ClusterError) {
	if gc.Stopped() {
		return nil, newClusterError("can not query groups info due to cluster gone",
			IssueMemberGoneError)
	}

	writerNames := gc.aliveNodesInCluster(WriteMode)
	reqParam := newRequestParam(writerNames, "", WriteMode, timeout)
	requestName := "(groups)query_groups"
	req := new(ReqQueryGroup)
	respPayloads, err, errType := gc.queryMultipleMembers(req, uint8(queryGroupMessage), requestName, reqParam)
	if err != nil {
		return nil, newClusterError(fmt.Sprintf("query groups info failed: %v", err), errType)
	}

	rets := make(map[string]*RespQueryGroup, len(respPayloads))
	for name := range respPayloads {
		var resp RespQueryGroup
		err = cluster.Unpack(respPayloads[name][1:], &resp)
		if err != nil {
			return nil, newClusterError(fmt.Sprintf("unpack query group response failed: %v", err),
				InternalServerError)
		}
		rets[name] = &resp
	}
	return rets, nil
}

// RetrieveGroup retrieve group detail information from writer node of specified group
func (gc *GatewayCluster) RetrieveGroup(group string, timeout time.Duration) (*RespQueryGroupPayload, *ClusterError) {
	if gc.Stopped() {
		return nil, newClusterError("can not query group info due to cluster gone",
			IssueMemberGoneError)
	}

	if !gc.groupExistInCluster(group) {
		return nil, newClusterError(fmt.Sprintf("query group failed, group %s doesn't exist", group), QueryGroupNotFoundError)
	}

	reqParam := newRequestParam(nil, group, WriteMode, timeout)
	requestName := fmt.Sprintf("(group:%s)query_group", group)
	req := new(ReqQueryGroup)
	respPayload, err, errType := gc.querySingleMember(req, uint8(queryGroupMessage), requestName, reqParam)
	if err != nil {
		return nil, newClusterError(fmt.Sprintf("query group(%s) info from writer failed: %v", group, err), errType)
	}

	var resp RespQueryGroup
	err = cluster.Unpack(respPayload[1:], &resp)
	if err != nil {
		return nil, newClusterError(fmt.Sprintf("unpack query group response failed: %v", err),
			InternalServerError)
	}

	return &resp.RespQueryGroupPayload, resp.Err
}

// RetrieveMembers retrieve member list from writer node of specified group
func (gc *GatewayCluster) RetrieveMembersList(group string, timeout time.Duration) (*RespQueryMembersList, *ClusterError) {
	if gc.Stopped() {
		return nil, newClusterError("can not query members list due to cluster gone",
			IssueMemberGoneError)
	}

	if !gc.groupExistInCluster(group) {
		return nil, newClusterError(fmt.Sprintf("query member lists failed, group %s doesn't exist", group), QueryGroupNotFoundError)
	}

	reqParam := newRequestParam(nil, group, WriteMode, timeout)
	requestName := fmt.Sprintf("(group:%s)query_members_list", group)
	req := new(ReqQueryMembersList)
	respPayload, err, errType := gc.querySingleMember(req, uint8(queryMembersListMessage), requestName, reqParam)
	if err != nil {
		return nil, newClusterError(fmt.Sprintf("query members list failed: %v", err), errType)
	}

	var resp RespQueryMembersList
	err = cluster.Unpack(respPayload[1:], &resp)
	if err != nil {
		return nil, newClusterError(fmt.Sprintf("unpack query members response failed: %v", err),
			InternalServerError)
	}

	return &resp, nil
}

// RetrieveMember retrieve member information from specified member
func (gc *GatewayCluster) RetrieveMember(group, nodeName string, timeout time.Duration) (*RespQueryMember, *ClusterError) {
	if gc.Stopped() {
		return nil, newClusterError("can not query member information due to cluster gone",
			IssueMemberGoneError)
	}

	// We can rely on member list information on group's writer node
	if gc.localGroupName() == group && gc.Mode() == WriteMode &&
		!common.StrInSlice(nodeName, gc.aliveNodesInCluster(NilMode)) {
		return nil, newClusterError(fmt.Sprintf("member %s doesn't alive", nodeName), QueryMemberNotFoundError)
	}

	reqParam := newRequestParam([]string{nodeName}, group, NilMode, timeout)
	requestName := fmt.Sprintf("(group:%s,member:%s)query_member_info", group, nodeName)
	req := new(ReqQuerySeq)
	respPayload, err, errType := gc.querySingleMember(req, uint8(queryMemberMessage), requestName, reqParam)
	if err != nil {
		return nil, newClusterError(fmt.Sprintf("query member %s information failed: %v", nodeName, err), errType)
	}

	var resp RespQueryMember
	err = cluster.Unpack(respPayload[1:], &resp)
	if err != nil {
		return nil, newClusterError(fmt.Sprintf("unpack query member response failed: %v", err),
			InternalServerError)
	}

	return &resp, nil
}

// health check

func (gc *GatewayCluster) checkGroupHealth(queryGroup *RespQueryGroupPayload) (GroupStatus, string) {
	status := Green
	var desc string
	writerExist := false
	for _, m := range queryGroup.MembersInfo.AliveMembers {
		if m.Mode == strings.ToLower(WriteMode.String()) {
			writerExist = true
			break
		}
	}
	if !writerExist {
		status = Red
		desc = "Writer node doesn't alive!"
	} else if !queryGroup.OpLogGroupInfo.OpLogStatus.Synced {
		status = Yellow
		desc = "some member(s)'s operation logs are unsynced!"
	}
	return status, desc
}

// RetrieveGroupHealthStatus retrieves health status of specified group
func (gc *GatewayCluster) RetrieveGroupHealthStatus(group string, timeout time.Duration) (*RespQueryGroupHealthPayload, *ClusterError) {
	groupInfo, err := gc.RetrieveGroup(group, timeout)
	if err != nil {
		return nil, err
	}
	status, desc := gc.checkGroupHealth(groupInfo)

	resp := &RespQueryGroupHealthPayload{
		ClusterResp: groupInfo.ClusterResp,
		Status:      status,
		Description: desc,
	}
	return resp, nil
}

func (gc *GatewayCluster) RetrieveClusterHealthStatus(timeout time.Duration) (*RespQueryGroupHealthPayload, *ClusterError) {
	groupsInfo, err := gc.RetrieveGroups(timeout)
	if err != nil {
		return nil, err
	}
	status := Green
	var desc string
	for name, groupInfo := range groupsInfo {
		if groupInfo.Err != nil {
			return nil, newClusterError(fmt.Sprintf("query group %s failed: %v", name, groupInfo.Err.Message), groupInfo.Err.Type)
		}
		s, d := gc.checkGroupHealth(&groupInfo.RespQueryGroupPayload)
		if s == Red {
			status = s
			desc = fmt.Sprintf("group(%s): %s", name, d)
			break
		} else if status == Green && s == Yellow {
			status = s
			desc = fmt.Sprintf("group(%s): %s", name, d)
		}
	}
	resp := &RespQueryGroupHealthPayload{
		ClusterResp: ClusterResp{common.Now()},
		Status:      status,
		Description: desc,
	}
	return resp, nil
}

// operation

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

	resp, err := gc.issueRetrieve(group, timeout, requestName, syncAll, &FilterRetrievePipelineTypes{})
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
func (gc *GatewayCluster) StatPipelineIndicatorNames(group string, timeout time.Duration, detail bool,
	pipelineName string) (StatResulter, *ClusterError) {

	if gc.Stopped() {
		return nil, newClusterError("can not retrieve pipeline statistics indicator names due to cluster gone",
			IssueMemberGoneError)
	}

	filter := FilterPipelineIndicatorNames{
		PipelineName: pipelineName,
	}

	requestName := fmt.Sprintf("(group(%s)stat_pipleine_indicator_names)", group)

	resp, err := gc.issueStat(group, timeout, detail, requestName, &filter)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (gc *GatewayCluster) StatPipelineIndicatorValue(group string, timeout time.Duration, detail bool,
	pipelineName, indicatorName string) (StatResulter, *ClusterError) {

	if gc.Stopped() {
		return nil, newClusterError("can not retrieve pipeline statistics indicator value due to cluster gone",
			IssueMemberGoneError)
	}

	filter := FilterPipelineIndicatorValue{
		PipelineName:  pipelineName,
		IndicatorName: indicatorName,
	}

	requestName := fmt.Sprintf("(group(%s)stat_pipleine_indicator_value)", group)

	resp, err := gc.issueStat(group, timeout, detail, requestName, &filter)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (gc *GatewayCluster) StatPipelineIndicatorsValue(group string, timeout time.Duration, detail bool,
	pipelineName string, indicatorNames []string) (StatResulter, *ClusterError) {

	if gc.Stopped() {
		return nil, newClusterError("can not retrieve pipeline statistics indicators value due to cluster gone",
			IssueMemberGoneError)
	}

	filter := FilterPipelineIndicatorsValue{
		PipelineName:   pipelineName,
		IndicatorNames: indicatorNames,
	}

	requestName := fmt.Sprintf("(group(%s)stat_pipleine_indicators_value)", group)

	resp, err := gc.issueStat(group, timeout, detail, requestName, &filter)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (gc *GatewayCluster) StatPipelineIndicatorDesc(group string, timeout time.Duration, detail bool,
	pipelineName, indicatorName string) (StatResulter, *ClusterError) {

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

	resp, err := gc.issueStat(group, timeout, detail, requestName, &filter)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (gc *GatewayCluster) StatPluginIndicatorNames(group string, timeout time.Duration, detail bool,
	pipelineName, pluginName string) (StatResulter, *ClusterError) {

	if gc.Stopped() {
		return nil, newClusterError("can not retrieve plugin statistics indicator names due to cluster gone",
			IssueMemberGoneError)
	}

	filter := FilterPluginIndicatorNames{
		PipelineName: pipelineName,
		PluginName:   pluginName,
	}

	requestName := fmt.Sprintf("(group(%s)stat_plugin_indicator_names)", group)

	resp, err := gc.issueStat(group, timeout, detail, requestName, &filter)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (gc *GatewayCluster) StatPluginIndicatorValue(group string, timeout time.Duration, detail bool,
	pipelineName, pluginName, indicatorName string) (StatResulter, *ClusterError) {

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

	resp, err := gc.issueStat(group, timeout, detail, requestName, &filter)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (gc *GatewayCluster) StatPluginIndicatorDesc(group string, timeout time.Duration, detail bool,
	pipelineName, pluginName, indicatorName string) (StatResulter, *ClusterError) {

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

	resp, err := gc.issueStat(group, timeout, detail, requestName, &filter)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (gc *GatewayCluster) StatTaskIndicatorNames(group string, timeout time.Duration, detail bool,
	pipelineName string) (StatResulter, *ClusterError) {

	if gc.Stopped() {
		return nil, newClusterError("can not retrieve task statistics indicator names due to cluster gone",
			IssueMemberGoneError)
	}

	filter := FilterTaskIndicatorNames{
		PipelineName: pipelineName,
	}

	requestName := fmt.Sprintf("(group(%s)stat_task_indicator_names)", group)

	resp, err := gc.issueStat(group, timeout, detail, requestName, &filter)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (gc *GatewayCluster) StatTaskIndicatorValue(group string, timeout time.Duration, detail bool,
	pipelineName, indicatorName string) (StatResulter, *ClusterError) {

	if gc.Stopped() {
		return nil, newClusterError("can not retrieve task statistics indicator value due to cluster gone",
			IssueMemberGoneError)
	}

	filter := FilterTaskIndicatorValue{
		PipelineName:  pipelineName,
		IndicatorName: indicatorName,
	}

	requestName := fmt.Sprintf("(group(%s)stat_task_indicator_value)", group)

	resp, err := gc.issueStat(group, timeout, detail, requestName, &filter)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (gc *GatewayCluster) StatTaskIndicatorDesc(group string, timeout time.Duration, detail bool,
	pipelineName, indicatorName string) (StatResulter, *ClusterError) {

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

	resp, err := gc.issueStat(group, timeout, detail, requestName, &filter)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
