package engine

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/hexdecteam/easegateway-types/pipelines"

	cluster "cluster/gateway"
	"common"
	"config"
	"logger"
	"model"
	"option"
	"plugins"
)

type Gateway struct {
	sync.Mutex
	repo       config.Store
	mod        *model.Model
	gc         *cluster.GatewayCluster
	schedulers map[string]pipelineScheduler
	done       chan error
	startAt    time.Time
}

func NewGateway() (*Gateway, error) {
	repo, err := config.InitStore()
	if err != nil {
		logger.Errorf("[initialize config repository failed: %v]", err)
		return nil, err
	}

	mod := model.NewModel()

	var memberMode cluster.Mode
	switch strings.ToLower(option.MemberMode) {
	case "read":
		memberMode = cluster.ReadMode
	case "write":
		memberMode = cluster.WriteMode
	default:
		return nil, fmt.Errorf("invalid member mode")
	}

	clusterConf := cluster.Config{
		ClusterGroup:      option.ClusterGroup,
		ClusterMemberMode: memberMode,
		ClusterMemberName: option.MemberName,
		Peers:             option.Peers,

		OPLogMaxSeqGapToPull:  option.OPLogMaxSeqGapToPull,
		OPLogPullMaxCountOnce: option.OPLogPullMaxCountOnce,
		OPLogPullInterval:     option.OPLogPullInterval,
		OPLogPullTimeout:      option.OPLogPullTimeout,
	}

	gc, err := cluster.NewGatewayCluster(clusterConf, mod)
	if err != nil {
		logger.Errorf("[create gateway cluster failed, clustering is disabled: %v]", err)
	}

	return &Gateway{
		repo:       repo,
		mod:        mod,
		gc:         gc,
		schedulers: make(map[string]pipelineScheduler, 100),
		done:       make(chan error, 1),
	}, nil
}

func (gw *Gateway) Close() {
	close(gw.done)
}

func (gw *Gateway) Run() (<-chan error, error) {
	if !gw.startAt.IsZero() {
		return nil, fmt.Errorf("gateway already started")
	}

	gw.startAt = time.Now()

	gw.setupPipelineLifecycleControl()

	err := gw.loadPlugins()
	if err != nil {
		return nil, err
	}

	err = gw.loadPipelines()
	if err != nil {
		return nil, err
	}

	gw.setupPluginPersistenceControl()
	gw.setupPipelinePersistenceControl()
	gw.setupClusterOpLogSync()

	return gw.done, nil
}

func (gw *Gateway) Stop() {
	gw.Lock()
	defer gw.Unlock()

	var err error

	if gw.gc != nil {
		logger.Infof("[closing gateway cluster]")

		err = gw.gc.Stop()
		if err != nil {
			logger.Errorf("[closing gateway cluster failed: %v]", err)
		} else {
			logger.Infof("[closed gateway cluster]")
		}
	}

	logger.Infof("[stopping pipelines]")

	for _, scheduler := range gw.schedulers {
		scheduler.Stop()
		scheduler.StopPipeline()
	}

	logger.Infof("[stopped pipelines]")

	logger.Infof("[cleaning and closing plugins]")
	gw.mod.DismissAllPluginInstances()
	logger.Infof("[cleaned and closed plugins]")

	logger.Infof("[close pipeline contexts]")

	for _, scheduler := range gw.schedulers {
		logger.Infof("[close pipeline context: %s]", scheduler.PipelineName())

		deleted := gw.mod.DeletePipelineContext(scheduler.PipelineName())
		if !deleted {
			logger.Errorf("[BUG: the pipeline %s has not context.]", scheduler.PipelineName())
		}
	}

	logger.Infof("[closed pipeline contexts]")

	gw.done <- err
}

func (gw *Gateway) Model() *model.Model {
	return gw.mod
}

func (gw *Gateway) Cluster() *cluster.GatewayCluster {
	return gw.gc
}

func (gw *Gateway) UpTime() time.Duration {
	if gw.startAt.IsZero() { // not started
		return 0
	} else {
		return time.Now().Sub(gw.startAt)
	}
}

func (gw *Gateway) SysAverageLoad() (load1, load5, load15 float64, err error) {
	err = fmt.Errorf("indicator not accessable")

	var e error

	line, e := ioutil.ReadFile("/proc/loadavg") // current support linux only
	if e != nil {
		return
	}

	values := strings.Fields(string(line))

	load1, e = strconv.ParseFloat(values[0], 64)
	if e != nil {
		return
	}

	load5, e = strconv.ParseFloat(values[1], 64)
	if e != nil {
		return
	}

	load15, e = strconv.ParseFloat(values[2], 64)
	if e != nil {
		return
	}

	err = nil
	return
}

func (gw *Gateway) SysResUsage() (*syscall.Rusage, error) {
	var resUsage syscall.Rusage
	err := syscall.Getrusage(0, // RUSAGE_SELF
		&resUsage)
	return &resUsage, err
}

func (gw *Gateway) setupPipelineLifecycleControl() {
	gw.mod.AddPipelineAddedCallback("launchPipeline", gw.launchPipeline,
		common.NORMAL_PRIORITY_CALLBACK)

	gw.mod.AddPipelineDeletedCallback("terminatePipeline", gw.terminatePipeline,
		common.NORMAL_PRIORITY_CALLBACK)
	gw.mod.AddPipelineDeletedCallback("closePipelineContext", gw.closePipelineContext,
		common.NORMAL_PRIORITY_CALLBACK)

	// separated to 2 stages for plugin cleanup on the pipeline context
	gw.mod.AddPipelineUpdatedCallback("relaunchPipelineStage1", gw.relaunchPipelineStage1,
		common.NORMAL_PRIORITY_CALLBACK)
	gw.mod.AddPipelineUpdatedCallback("relaunchPipelineStage2", gw.relaunchPipelineStage2,
		common.NORMAL_PRIORITY_CALLBACK)
}

func (gw *Gateway) launchPipeline(newPipeline *model.Pipeline) {
	if newPipeline.Config().Parallelism() == 0 { // dynamic mode
		logger.Infof("[launch pipeline: %s (parallelism=dynamic)]", newPipeline.Name())
	} else {
		logger.Infof("[launch pipeline: %s (parallelism=%d)]", newPipeline.Name(), newPipeline.Config().Parallelism())
	}

	gw.Lock()
	defer gw.Unlock()

	statistics := gw.mod.StatRegistry().GetPipelineStatistics(newPipeline.Name())
	if statistics == nil {
		logger.Errorf("[launch pipeline %s failed: pipeline statistics not found]", newPipeline.Name())
		return
	}

	var scheduler pipelineScheduler

	if newPipeline.Config().Parallelism() == 0 { // dynamic mode
		scheduler = newDynamicPipelineScheduler(newPipeline)
	} else { // pre-alloc mode
		scheduler = newStaticPipelineScheduler(newPipeline)
	}

	ctx := gw.mod.CreatePipelineContext(newPipeline.Config(), statistics, scheduler.SourceInputTrigger())

	pluginNames := newPipeline.Config().PluginNames()
	preparedPlugins := uint32(len(pluginNames))

	gw.mod.AddPluginUpdatedCallback(fmt.Sprintf("%s-rePreparePlugin", newPipeline.Name()),
		gw.getRePreparePluginCallback(pluginNames, &preparedPlugins, ctx),
		common.NORMAL_PRIORITY_CALLBACK)

	scheduler.Start(ctx, statistics, gw.mod, &preparedPlugins)

	gw.schedulers[newPipeline.Name()] = scheduler
}

func (gw *Gateway) relaunchPipelineStage1(updatedPipeline *model.Pipeline) {
	gw.terminatePipeline(updatedPipeline)
}

func (gw *Gateway) relaunchPipelineStage2(updatedPipeline *model.Pipeline) {
	gw.closePipelineContext(updatedPipeline)
	gw.launchPipeline(updatedPipeline)
}

func (gw *Gateway) terminatePipeline(deletedPipeline *model.Pipeline) {
	logger.Infof("[terminate pipeline: %s]", deletedPipeline.Name())

	gw.Lock()
	defer gw.Unlock()

	scheduler, exists := gw.schedulers[deletedPipeline.Name()]
	if !exists {
		logger.Errorf("[BUG: deleted pipeline %s didn't launched.]", deletedPipeline.Name())
		return
	}

	scheduler.Stop()

	scheduler.StopPipeline()

	gw.mod.DeletePluginUpdatedCallback(fmt.Sprintf("%s-rePreparePlugin", deletedPipeline.Name()))

	delete(gw.schedulers, deletedPipeline.Name())
}

func (gw *Gateway) closePipelineContext(deletedPipeline *model.Pipeline) {
	logger.Infof("[close pipeline context: %s]", deletedPipeline.Name())

	deleted := gw.mod.DeletePipelineContext(deletedPipeline.Name())
	if !deleted {
		logger.Errorf("[BUG: deleted pipeline %s has not context.]", deletedPipeline.Name())
		return
	}
}

func (gw *Gateway) loadPlugins() error {
	specs, err := gw.repo.GetAllPlugins()
	if err != nil {
		logger.Errorf("[load plugins from storage failed: %v]", err)
		return err
	}

	err = gw.mod.LoadPlugins(specs)
	if err != nil {
		logger.Errorf("[load model from plugin repository failed: %v]", err)
		return err
	}

	logger.Infof("[plugins are loaded from repository successfully (total=%d)]", len(specs))

	return nil
}

func (gw *Gateway) loadPipelines() error {
	specs, err := gw.repo.GetAllPipelines()
	if err != nil {
		logger.Errorf("[load pipelines from storage failed: %v]", err)
		return err
	}

	err = gw.mod.LoadPipelines(specs)
	if err != nil {
		logger.Errorf("[load model form pipeline repository failed: %v]", err)
		return err
	}

	logger.Infof("[pipelines are loaded from repository successfully (total=%d)]", len(specs))

	return nil
}

func (gw *Gateway) setupPluginPersistenceControl() {
	gw.mod.AddPluginAddedCallback("addPluginToStorage", gw.addPluginToStorage,
		common.NORMAL_PRIORITY_CALLBACK)
	gw.mod.AddPluginDeletedCallback("deletePluginFromStorage", gw.deletePluginFromStorage,
		common.NORMAL_PRIORITY_CALLBACK)
	gw.mod.AddPluginUpdatedCallback("updatePluginInStorage", gw.updatePluginInStorage,
		common.NORMAL_PRIORITY_CALLBACK)
}

func (gw *Gateway) setupPipelinePersistenceControl() {
	gw.mod.AddPipelineAddedCallback("addPipelineToStorage", gw.addPipelineToStorage,
		common.NORMAL_PRIORITY_CALLBACK)
	gw.mod.AddPipelineDeletedCallback("deletePipelineFromStorage", gw.deletePipelineFromStorage,
		common.NORMAL_PRIORITY_CALLBACK)
	gw.mod.AddPipelineUpdatedCallback("updatePipelineInStorage", gw.updatePipelineInStorage,
		common.NORMAL_PRIORITY_CALLBACK)
}

func (gw *Gateway) setupClusterOpLogSync() {
	if gw.gc != nil {
		gw.gc.OPLog().AddOPLogAppendedCallback("handleClusterOperation", gw.handleClusterOperation,
			common.NORMAL_PRIORITY_CALLBACK)
	}
}

func (gw *Gateway) addPluginToStorage(newPlugin *model.Plugin) {
	spec := &config.PluginSpec{
		Type:   newPlugin.Type(),
		Config: newPlugin.Config(),
	}

	err := gw.repo.AddPlugin(spec)
	if err != nil {
		logger.Errorf("[add plugin %s failed: %v]", newPlugin.Name(), err)
	}
}

func (gw *Gateway) deletePluginFromStorage(deletedPlugin *model.Plugin) {
	err := gw.repo.DeletePlugin(deletedPlugin.Name())
	if err != nil {
		logger.Errorf("[delete plugin %s failed: %v]", deletedPlugin.Name(), err)
	}
}

func (gw *Gateway) updatePluginInStorage(updatedPlugin *model.Plugin) {
	spec := &config.PluginSpec{
		Type:   updatedPlugin.Type(),
		Config: updatedPlugin.Config(),
	}

	err := gw.repo.UpdatePlugin(spec)
	if err != nil {
		logger.Errorf("[update plugin %s failed: %v]", updatedPlugin.Name(), err)
	}
}

func (gw *Gateway) addPipelineToStorage(newPipeline *model.Pipeline) {
	spec := &config.PipelineSpec{
		Type:   newPipeline.Type(),
		Config: newPipeline.Config(),
	}
	err := gw.repo.AddPipeline(spec)
	if err != nil {
		logger.Errorf("[add pipeline %s failed: %v]", newPipeline.Name(), err)
	}
}

func (gw *Gateway) deletePipelineFromStorage(deletedPipeline *model.Pipeline) {
	err := gw.repo.DeletePipeline(deletedPipeline.Name())
	if err != nil {
		logger.Errorf("[delete pipeline %s failed: %v]", deletedPipeline.Name(), err)
	}
}

func (gw *Gateway) updatePipelineInStorage(updatedPipeline *model.Pipeline) {
	spec := &config.PipelineSpec{
		Type:   updatedPipeline.Type(),
		Config: updatedPipeline.Config(),
	}

	err := gw.repo.UpdatePipeline(spec)
	if err != nil {
		logger.Errorf("[update pipeline %s failed: %v]", updatedPipeline.Name(), err)
	}
}

func (gw *Gateway) handleClusterOperation(seq uint64, operation *cluster.Operation) (
	error, cluster.OperationFailureType) {

	switch {
	case operation.ContentCreatePlugin != nil:
		content := operation.ContentCreatePlugin

		conf, err := plugins.GetConfig(content.Type)
		if err != nil {
			logger.Errorf("[handle cluster operation to create plugin failed on get config: %v]", err)
			return err, cluster.OperationGeneralFailure
		}

		err = json.Unmarshal(content.Config, conf)
		if err != nil {
			logger.Errorf("[handle cluster operation to create plugin failed on unmarshal config: %v]", err)
			return err, cluster.OperationGeneralFailure
		}

		pluginName := conf.PluginName()

		plugin, _ := gw.mod.GetPlugin(pluginName)
		if plugin != nil {
			logger.Errorf("[handle cluster operation to create plugin failed: plugin %s already exists]",
				pluginName)
			return fmt.Errorf("plugin %s already exists", pluginName), cluster.OperationConflictFailure
		}

		constructor, err := plugins.GetConstructor(content.Type)
		if err != nil {
			logger.Errorf("[handle cluster operation to create plugin failed on get constructor: %v]", err)
			return err, cluster.OperationGeneralFailure
		}

		_, err = gw.mod.AddPlugin(content.Type, conf, constructor)
		if err != nil {
			logger.Errorf("[handle cluster operation to create plugin failed on add to model: %v]", err)
			return err, cluster.OperationGeneralFailure
		}
	case operation.ContentUpdatePlugin != nil:
		content := operation.ContentUpdatePlugin

		conf, err := plugins.GetConfig(content.Type)
		if err != nil {
			logger.Errorf("[handle cluster operation to update plugin failed on get config: %v]", err)
			return err, cluster.OperationGeneralFailure
		}

		err = json.Unmarshal(content.Config, conf)
		if err != nil {
			logger.Errorf("[handle cluster operation to update plugin failed on unmarshal config: %v]",
				err)
			return err, cluster.OperationGeneralFailure
		}

		pluginName := conf.PluginName()

		plugin, _ := gw.mod.GetPlugin(pluginName)
		if plugin == nil {
			logger.Errorf("[handle cluster operation to update plugin failed: plugin %s not found]",
				pluginName)
			return fmt.Errorf("plugin %s not found", pluginName), cluster.OperationTargetNotFoundFailure
		}

		if plugin.Type() != content.Type {
			logger.Errorf("[handle cluster operation to update plugin failed: plugin type %s is readonly]",
				plugin.Type())
			return fmt.Errorf("plugin type %s is readonly", plugin.Type()), cluster.OperationGeneralFailure
		}

		err = gw.mod.UpdatePluginConfig(conf)
		if err != nil {
			logger.Errorf("[handle cluster operation to update plugin failed on update model: %v]", err)
			return err, cluster.OperationGeneralFailure
		}
	case operation.ContentDeletePlugin != nil:
		content := operation.ContentDeletePlugin

		plugin, refCount := gw.mod.GetPlugin(content.Name)
		if plugin == nil {
			logger.Errorf("[handle cluster operation to delete plugin failed: plugin %s not found]",
				content.Name)
			return fmt.Errorf("plugin %s not found", content.Name), cluster.OperationTargetNotFoundFailure
		}

		if refCount > 0 {
			logger.Errorf("[handle cluster operation to delete plugin failed: "+
				"plugin %s is used by %d pipeline(s)]", content.Name, refCount)
			return fmt.Errorf("plugin %s is used by one or more pipelines", content.Name),
				cluster.OperationNotAcceptableFailure
		}

		err := gw.mod.DismissPluginInstanceByName(content.Name)
		if err != nil {
			logger.Errorf("[handle cluster operation to delete plugin failed on "+
				"dismiss plugin instance on model: %v]", err)
			return err, cluster.OperationUnknownFailure
		}

		err = gw.mod.DeletePlugin(content.Name)
		if err != nil {
			logger.Errorf("[handle cluster operation to delete plugin failed on delete from model: %v]",
				err)
			return err, cluster.OperationNotAcceptableFailure
		}
	case operation.ContentCreatePipeline != nil:
		content := operation.ContentCreatePipeline

		conf, err := model.GetPipelineConfig(content.Type)
		if err != nil {
			logger.Errorf("[handle cluster operation to create pipeline failed on get config: %v]", err)
			return err, cluster.OperationGeneralFailure
		}

		err = json.Unmarshal(content.Config, conf)
		if err != nil {
			logger.Errorf("[handle cluster operation to create pipeline failed on unmarshal config: %v]",
				err)
			return err, cluster.OperationGeneralFailure
		}

		pipelineName := conf.PipelineName()

		pipeline := gw.mod.GetPipeline(pipelineName)
		if pipeline != nil {
			logger.Errorf("[handle cluster operation to create pipeline failed: "+
				"pipeline %s already exists]", pipelineName)
			return fmt.Errorf("pipeline %s already exists", pipelineName), cluster.OperationConflictFailure
		}

		_, err = gw.mod.AddPipeline(content.Type, conf)
		if err != nil {
			logger.Errorf("[handle cluster operation to create pipeline failed on add to model: %v]", err)
			return err, cluster.OperationGeneralFailure
		}
	case operation.ContentUpdatePipeline != nil:
		content := operation.ContentUpdatePipeline

		conf, err := model.GetPipelineConfig(content.Type)
		if err != nil {
			logger.Errorf("[handle cluster operation to update pipeline failed on get config: %v]", err)
			return err, cluster.OperationGeneralFailure
		}

		err = json.Unmarshal(content.Config, conf)
		if err != nil {
			logger.Errorf("[handle cluster operation to update pipeline failed on unmarshal config: %v]",
				err)
			return err, cluster.OperationGeneralFailure
		}

		pipelineName := conf.PipelineName()

		pipeline := gw.mod.GetPipeline(pipelineName)
		if pipeline == nil {
			logger.Errorf("[handle cluster operation to update pipeline failed: pipeline %s not found]",
				pipelineName)
			return fmt.Errorf("pipeline %s not found", pipelineName),
				cluster.OperationTargetNotFoundFailure
		}

		if pipeline.Type() != content.Type {
			logger.Errorf("[handle cluster operation to update pipeline failed: "+
				"pipeline type %s is readonly]", pipeline.Type())
			return fmt.Errorf("pipeline type %s is readonly", pipeline.Type()),
				cluster.OperationGeneralFailure
		}

		err = gw.mod.UpdatePipelineConfig(conf)
		if err != nil {
			logger.Errorf("[handle cluster operation to update pipeline failed on update model: %v]", err)
			return err, cluster.OperationGeneralFailure
		}
	case operation.ContentDeletePipeline != nil:
		content := operation.ContentDeletePipeline

		pipeline := gw.mod.GetPipeline(content.Name)
		if pipeline == nil {
			logger.Errorf("[handle cluster operation to delete pipeline failed: pipeline %s not found]",
				content.Name)
			return fmt.Errorf("pipeline %s not found", content.Name),
				cluster.OperationTargetNotFoundFailure
		}

		err := gw.mod.DeletePipeline(content.Name)
		if err != nil {
			logger.Errorf("[handle cluster operation to delete pipeline failed on delete from model: %v]",
				err)
			return err, cluster.OperationGeneralFailure
		}
	default:
		logger.Errorf("[BUG: cluster operation (sequence=%d) has no certain content, skipped]", seq)

		return fmt.Errorf("cluster operation (sequence=%d) has no certain content", seq),
			cluster.OperationUnknownFailure
	}

	logger.Debugf("[cluster operation (sequence=%d) has been handled]", seq)

	return nil, cluster.NoneOperationFailure
}

func (gw *Gateway) getRePreparePluginCallback(pluginNames []string, preparedPlugins *uint32,
	ctx pipelines.PipelineContext) model.PluginUpdated {

	return func(updatedPlugin *model.Plugin) {
		if !common.StrInSlice(updatedPlugin.Name(), pluginNames) {
			return
		}

		atomic.StoreUint32(preparedPlugins, 0)
	}
}
