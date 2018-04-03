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
var NoneGroup = ""

// RetrieveGroupsList retrieve groups list from current node
func (gc *GatewayCluster) RetrieveGroupsList() ([]string, *ClusterError) {
	if gc.Stopped() {
		return nil, newClusterError("can not query group info due to cluster gone",
			IssueMemberGoneError)
	}

	groups := gc.groupsInCluster()

	return groups, nil
}

// RetrieveGroups retrieve all groups' detail information from group's corresponding writer node
// return result `map[string]*RespQueryGroup`, key is corresponding writer's member name
// may return non-nil `map[string]*RespQueryGroup` when partially complete, so caller can deal with the payloads
func (gc *GatewayCluster) RetrieveGroups(timeout time.Duration) (map[string]*RespQueryGroup, *ClusterError) {
	if gc.Stopped() {
		return nil, newClusterError("can not query groups info due to cluster gone",
			IssueMemberGoneError)
	}

	writers, err := gc.writerInEveryGroup()
	if err != nil {
		return nil, newClusterError(fmt.Sprintf("query groups failed: %v", err), QueryMemberNotFoundError)
	}

	reqParam := newRequestParam(writers, NoneGroup, WriteMode, timeout)
	requestName := "(groups)query_groups"
	req := new(ReqQueryGroup)
	respPayloads, err, errType := gc.queryMultipleMembers(req, uint8(queryGroupMessage), requestName, reqParam)
	var retErr *ClusterError
	if err != nil {
		if errType == QueryPartiallyCompleteError {
			retErr = newClusterError(err.Error(), errType)
		} else {
			return nil, newClusterError(fmt.Sprintf("query groups info failed: %v", err), errType)
		}
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
	return rets, retErr
}

// RetrieveGroup retrieve group detail information from writer node of specified group
// If return parameter of *ClusterError.Type is QueryPartiallyCompleteError, then coordinate error
// msg contains the timeout nodes
func (gc *GatewayCluster) RetrieveGroup(group string, timeout time.Duration) (*RespQueryGroupPayload, *ClusterError) {
	logger.Infof("retrieve group %s, timeout: %.3f", group, timeout.Seconds())
	if gc.Stopped() {
		return nil, newClusterError("can not query group info due to cluster gone",
			IssueMemberGoneError)
	}

	if !gc.groupExistInCluster(group) {
		return nil, newClusterError(fmt.Sprintf("query group failed, group %s doesn't exist", group), QueryGroupNotFoundError)
	}
	node, err := gc.writerInGroup(group)
	if err != nil {
		return nil, newClusterError(fmt.Sprintf("query group failed: %v", err), QueryMemberNotFoundError)
	}
	reqParam := newRequestParam([]string{node}, group, WriteMode, timeout)
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
	if resp.Err != nil {
		resp.RespQueryGroupPayload.TimeoutNodes = strings.Split(resp.Err.Message, ",")
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
	node, err := gc.choosePeerForGroup(group)
	if err != nil {
		return nil, newClusterError(fmt.Sprintf("query member lists for group %s failed: %v ", group, err), QueryMemberNotFoundError)
	}
	reqParam := newRequestParam([]string{node}, group, NilMode, timeout)
	requestName := fmt.Sprintf("(group:%s)query_members_list", group)
	req := new(ReqQueryMembersList)
	respPayload, err, errType := gc.querySingleMember(req, uint8(queryMembersListMessage), requestName, reqParam)
	if err != nil {
		return nil, newClusterError(fmt.Sprintf("query members list from writer failed: %v", err), errType)
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
		!common.StrInSlice(nodeName, gc.aliveNodesInCluster(NilMode, group)) {
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
	var retErr *ClusterError
	if err != nil {
		if err.Type != QueryPartiallyCompleteError {
			return nil, err
		}
		retErr = err
	}
	status, desc := gc.checkGroupHealth(groupInfo)

	resp := &RespQueryGroupHealthPayload{
		ClusterResp: groupInfo.ClusterResp,
		Status:      status,
		Description: desc,
	}
	if retErr != nil {
		resp.TimeoutNodes = strings.Split(retErr.Message, ",")
	}
	return resp, nil
}

func (gc *GatewayCluster) RetrieveClusterHealthStatus(timeout time.Duration) (*RespQueryGroupHealthPayload, *ClusterError) {
	groupsInfo, err := gc.RetrieveGroups(timeout)
	var timeoutNodes string
	if err != nil {
		if err.Type != QueryPartiallyCompleteError {
			return nil, err
		} else {
			timeoutNodes = err.Message
		}
	}
	status := Green
	var desc string
	for writerName, groupInfo := range groupsInfo {
		if groupInfo.Err != nil {
			if groupInfo.Err.Type != QueryPartiallyCompleteError {
				return nil, newClusterError(fmt.Sprintf("query group from writer %s failed: %v", writerName, groupInfo.Err.Message), groupInfo.Err.Type)
			} else {
				timeoutNodes = timeoutNodes + "," + groupInfo.Err.Message
			}
		}
		s, d := gc.checkGroupHealth(&groupInfo.RespQueryGroupPayload)
		if s != Green { // choose more severe status
			if len(desc) > 0 {
				desc = fmt.Sprintf("%s; group(%s): %s", desc, groupInfo.RespQueryGroupPayload.GroupName, d)
			} else {
				desc = fmt.Sprintf("group(%s): %s", groupInfo.RespQueryGroupPayload.GroupName, d)
			}
			if status == Green {
				status = s
			} else if status != Red || s == Red {
				status = s
			}
		}
	}
	resp := &RespQueryGroupHealthPayload{
		ClusterResp: ClusterResp{common.Now()},
		Status:      status,
		Description: desc,
	}

	if timeoutNodes != "" {
		resp.TimeoutNodes = strings.Split(timeoutNodes, ",")
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

	resp, err := gc.issueRetrieve(group, timeout, requestName, syncAll, &filter,
		common.DEFAULT_PAGE, common.DEFAULT_LIMIT_PER_PAGE)
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
	namePattern string, types []string, page, limit uint32) (*ResultRetrievePlugins, *ClusterError) {

	if gc.Stopped() {
		return nil, newClusterError("can not retrieve plugins due to cluster gone", IssueMemberGoneError)
	}

	filter := FilterRetrievePlugins{
		NamePattern: namePattern,
		Types:       types,
	}

	requestName := fmt.Sprintf("(group:%s)retrive_plugins", group)

	resp, err := gc.issueRetrieve(group, timeout, requestName, syncAll, &filter, page, limit)
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

	resp, err := gc.issueRetrieve(group, timeout, requestName, syncAll, &filter,
		common.DEFAULT_PAGE, common.DEFAULT_LIMIT_PER_PAGE)
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
	namePattern string, types []string, page, limit uint32) (*ResultRetrievePipelines, *ClusterError) {

	if gc.Stopped() {
		return nil, newClusterError("can not retrieve pipelines due to cluster gone", IssueMemberGoneError)
	}

	filter := FilterRetrievePipelines{
		NamePattern: namePattern,
		Types:       types,
	}

	requestName := fmt.Sprintf("(group:%s)retrive_pipelines", group)

	resp, err := gc.issueRetrieve(group, timeout, requestName, syncAll, &filter, page, limit)
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

	resp, err := gc.issueRetrieve(group, timeout, requestName, syncAll, &FilterRetrievePluginTypes{},
		common.DEFAULT_PAGE, common.DEFAULT_LIMIT_PER_PAGE)
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

	resp, err := gc.issueRetrieve(group, timeout, requestName, syncAll, &FilterRetrievePipelineTypes{},
		common.DEFAULT_PAGE, common.DEFAULT_LIMIT_PER_PAGE)
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
	pipelineName string) (StatResult, *ClusterError) {

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
	pipelineName, indicatorName string) (StatResult, *ClusterError) {

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
	pipelineName string, indicatorNames []string) (StatResult, *ClusterError) {

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
	pipelineName, indicatorName string) (StatResult, *ClusterError) {

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
	pipelineName, pluginName string) (StatResult, *ClusterError) {

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
	pipelineName, pluginName, indicatorName string) (StatResult, *ClusterError) {

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
	pipelineName, pluginName, indicatorName string) (StatResult, *ClusterError) {

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
	pipelineName string) (StatResult, *ClusterError) {

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
	pipelineName, indicatorName string) (StatResult, *ClusterError) {

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
	pipelineName, indicatorName string) (StatResult, *ClusterError) {

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
