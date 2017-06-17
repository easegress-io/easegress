package engine

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	cluster "cluster/gateway"
	"config"
	"logger"
	"model"
	"pipelines"
	"plugins"
)

const (
	PIPELINE_STOP_TIMEOUT_SECONDS = 30
)

type pipelineInstance struct {
	instance pipelines.Pipeline
	stop     chan struct{}
	done     chan struct{}
}

func newPipelineInstance(instance pipelines.Pipeline) *pipelineInstance {
	return &pipelineInstance{
		instance: instance,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

func (pi *pipelineInstance) run() {
loop:
	for {
		select {
		case <-pi.stop:
			break loop
		default:
			err := pi.instance.Run()
			if err != nil {
				logger.Errorf(
					"[pipeline %s runs error and exits exceptionally: %v]",
					pi.instance.Name(), err)
				break loop
			}
		}
	}

	pi.instance.Close()
	close(pi.done)
}

// use <-pi.terminate() to wait some time
// use   pi.terminate() to leave it alone
func (pi *pipelineInstance) terminate() chan struct{} {
	pi.instance.Stop()
	close(pi.stop)

	return pi.done
}

type Gateway struct {
	sync.Mutex
	repo      config.Store
	mod       *model.Model
	gc        *cluster.GatewayCluster
	pipelines map[string][]*pipelineInstance
	done      chan error
	startAt   time.Time
}

func NewGateway() (*Gateway, error) {
	repo, err := config.InitStore()
	if err != nil {
		logger.Errorf("[initialize config repository failed: %v]", err)
		return nil, err
	}

	mod := model.NewModel()

	// TODO: read from launch config
	clusterConf := cluster.Config{
		OPLogMaxSeqGapToPull:  5,
		OPLogPullMaxCountOnce: 5,
		OPLogPullInterval:     10 * time.Second,
		OPLogPullTimeout:      30 * time.Second,
	}

	gc, err := cluster.NewGatewayCluster(clusterConf, mod)
	if err != nil {
		return nil, fmt.Errorf("[create gateway cluster failed: %v]", err)
	}

	return &Gateway{
		repo:      repo,
		mod:       mod,
		gc:        gc,
		pipelines: make(map[string][]*pipelineInstance),
		done:      make(chan error, 1),
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

	logger.Infof("[stopping pipelines]")

	for name, pipes := range gw.pipelines {
		logger.Debugf("[stopping pipeline %s]", name)

		for i, pi := range pipes {
			select {
			case <-pi.terminate():
			case <-time.After(PIPELINE_STOP_TIMEOUT_SECONDS * time.Second):
				logger.Warnf("[stopped pipeline %s-#%d timeout (%d seconds)]",
					name, i+1, PIPELINE_STOP_TIMEOUT_SECONDS)
			}
		}

		logger.Debugf("[stopped pipeline %s]", name)
	}

	logger.Infof("[stopped pipelines]")

	logger.Infof("[closing plugins]")
	gw.mod.DismissAllPluginInstances()
	logger.Infof("[closed plugins]")

	logger.Infof("[closing gateway cluster]")
	err := gw.gc.Stop()
	if err != nil {
		logger.Errorf("[closing gateway cluster failed: %v]", err)
	} else {
		logger.Infof("[closed gateway cluster]")
	}

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
	gw.mod.AddPipelineAddedCallback("launchPipeline", gw.launchPipeline, false)
	gw.mod.AddPipelineDeletedCallback("terminatePipeline", gw.terminatePipeline, false)
	gw.mod.AddPipelineUpdatedCallback("relaunchPipeline", gw.relaunchPipeline, false)
}

func (gw *Gateway) launchPipeline(newPipeline *model.Pipeline) {
	logger.Infof("[launch pipeline: %s (parallelism=%d)]", newPipeline.Name(), newPipeline.Config().Parallelism())

	gw.Lock()
	defer gw.Unlock()

	statistics := gw.mod.StatRegistry().GetPipelineStatistics(newPipeline.Name())
	if statistics == nil {
		logger.Errorf("[launch pipeline %s failed: pipeline statistics not found]", newPipeline.Name())
		return
	}

	ctx := gw.mod.CreatePipelineContext(newPipeline.Config(), statistics)

	for i := uint16(0); i < newPipeline.Config().Parallelism(); i++ {
		instance, err := newPipeline.GetInstance(ctx, statistics, gw.mod)
		if err != nil {
			logger.Errorf("[launch pipeline %s-#%d failed: %v]", newPipeline.Name(), i, err)
			return
		}

		p := newPipelineInstance(instance)

		go p.run()

		pipes := gw.pipelines[newPipeline.Name()]
		pipes = append(pipes, p)
		gw.pipelines[newPipeline.Name()] = pipes
	}
}

func (gw *Gateway) relaunchPipeline(updatedPipeline *model.Pipeline) {
	gw.terminatePipeline(updatedPipeline)
	gw.launchPipeline(updatedPipeline)
}

func (gw *Gateway) terminatePipeline(deletedPipeline *model.Pipeline) {
	logger.Infof("[terminate pipeline: %s]", deletedPipeline.Name())

	gw.Lock()
	defer gw.Unlock()

	pipes, exists := gw.pipelines[deletedPipeline.Name()]
	if !exists {
		logger.Errorf("[BUG: deleted pipeline %s didn't launched.]", deletedPipeline.Name())
		return
	}

	for _, pi := range pipes {
		<-pi.terminate()
	}

	delete(gw.pipelines, deletedPipeline.Name())

	deleted := gw.mod.DeletePipelineContext(deletedPipeline.Name())
	if !deleted {
		logger.Errorf("[BUG: deleted pipeline %s have not context.]", deletedPipeline.Name())
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

	logger.Infof("[plugins are loaded successfully (total=%d)]", len(specs))

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

	logger.Infof("[pipelines are loaded successfully (total=%d)]", len(specs))

	return nil
}

func (gw *Gateway) setupPluginPersistenceControl() {
	gw.mod.AddPluginAddedCallback("addPluginToStorage", gw.addPluginToStorage, false)
	gw.mod.AddPluginDeletedCallback("deletePluginFromStorage", gw.deletePluginFromStorage, false)
	gw.mod.AddPluginUpdatedCallback("updatePluginInStorage", gw.updatePluginInStorage, false)
}

func (gw *Gateway) setupPipelinePersistenceControl() {
	gw.mod.AddPipelineAddedCallback("addPipelineToStorage", gw.addPipelineToStorage, false)
	gw.mod.AddPipelineDeletedCallback("deletePipelineFromStorage", gw.deletePipelineFromStorage, false)
	gw.mod.AddPipelineUpdatedCallback("updatePipelineInStorage", gw.updatePipelineInStorage, false)
}

func (gw *Gateway) setupClusterOpLogSync() {
	gw.gc.OPLog().AddOPLogAppendedCallback("handleClusterOperation", gw.handleClusterOperation, false)
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

func (gw *Gateway) handleClusterOperation(seq uint64, operation *cluster.Operation) {
	switch {
	case operation.ContentCreatePlugin != nil:
		content := operation.ContentCreatePlugin

		conf, err := plugins.GetConfig(content.Type)
		if err != nil {
			logger.Errorf("[handle cluster operation to create plugin failed on get config: %v]", err)
			return
		}

		err = json.Unmarshal(content.Config, conf)
		if err != nil {
			logger.Errorf("[handle cluster operation to create plugin failed on unmarshal config: %v]",
				err)
			return
		}

		constructor, err := plugins.GetConstructor(content.Type)
		if err != nil {
			logger.Errorf("[handle cluster operation to create plugin failed on get constructor: %v]",
				err)
			return
		}

		_, err = gw.mod.AddPlugin(content.Type, conf, constructor)
		if err != nil {
			logger.Errorf("[handle cluster operation to create plugin failed on add to model: %v]", err)
			return
		}
	case operation.ContentUpdatePlugin != nil:
		content := operation.ContentUpdatePlugin

		conf, err := plugins.GetConfig(content.Type)
		if err != nil {
			logger.Errorf("[handle cluster operation to update plugin failed on get config: %v]", err)
			return
		}

		err = json.Unmarshal(content.Config, conf)
		if err != nil {
			logger.Errorf("[handle cluster operation to update plugin failed on unmarshal config: %v]",
				err)
			return
		}

		err = gw.mod.UpdatePluginConfig(conf)
		if err != nil {
			logger.Errorf("[handle cluster operation to update plugin failed on update model: %v]", err)
			return
		}
	case operation.ContentDeletePlugin != nil:
		content := operation.ContentDeletePlugin

		err := gw.mod.DeletePlugin(content.Name)
		if err != nil {
			logger.Errorf("[handle cluster operation to delete plugin failed on delete from model: %v]",
				err)
			return
		}
	case operation.ContentCreatePipeline != nil:
		content := operation.ContentCreatePipeline

		conf, err := model.GetPipelineConfig(content.Type)
		if err != nil {
			logger.Errorf("[handle cluster operation to create pipeline failed on get config: %v]", err)
			return
		}

		err = json.Unmarshal(content.Config, conf)
		if err != nil {
			logger.Errorf("[handle cluster operation to create pipeline failed on unmarshal  config: %v]",
				err)
			return
		}

		_, err = gw.mod.AddPipeline(content.Type, conf)
		if err != nil {
			logger.Errorf("[handle cluster operation to create pipeline failed on add to model: %v]", err)
			return
		}
	case operation.ContentUpdatePipeline != nil:
		content := operation.ContentUpdatePipeline

		conf, err := model.GetPipelineConfig(content.Type)
		if err != nil {
			logger.Errorf("[handle cluster operation to update pipeline failed on get config: %v]", err)
			return
		}

		err = json.Unmarshal(content.Config, conf)
		if err != nil {
			logger.Errorf("[handle cluster operation to update pipeline failed on unmarshal config: %v]",
				err)
			return
		}

		err = gw.mod.UpdatePipelineConfig(conf)
		if err != nil {
			logger.Errorf("[handle cluster operation to update pipeline failed on update model: %v]", err)
			return
		}
	case operation.ContentDeletePipeline != nil:
		content := operation.ContentDeletePipeline

		err := gw.mod.DeletePipeline(content.Name)
		if err != nil {
			logger.Errorf("[handle cluster operation to delete pipeline failed on delete from model: %v]",
				err)
			return
		}
	default:
		logger.Errorf("[BUG: cluster operation (sequence=%d) has no certain content, skipped]", seq)
		return
	}

	logger.Debugf("[cluster operation (sequence=%d) has been handled]", seq)
}
