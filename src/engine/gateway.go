package engine

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"config"
	"logger"
	"model"
	"pipelines"
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
					"[pipeline %s runs error and exits exceptionally: %s]",
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
	repo             config.Store
	mod              *model.Model
	pipelines        map[string][]*pipelineInstance
	pipelineContexts map[string]pipelines.PipelineContext
	done             chan error
	startAt          time.Time
}

func NewGateway() (*Gateway, error) {
	repo, err := config.InitStore()
	if err != nil {
		logger.Errorf("[initialize config repository failed: %v]", err)
		return nil, err
	}

	mod := model.NewModel()

	return &Gateway{
		repo:             repo,
		mod:              mod,
		pipelines:        make(map[string][]*pipelineInstance),
		pipelineContexts: make(map[string]pipelines.PipelineContext),
		done:             make(chan error, 1),
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

	gw.done <- nil
}
func (gw *Gateway) Model() *model.Model {
	return gw.mod
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

	ctx := model.NewPipelineContext(newPipeline.Config(), statistics, gw.mod)
	gw.pipelineContexts[newPipeline.Name()] = ctx

	for i := uint16(0); i < newPipeline.Config().Parallelism(); i++ {
		instance, err := newPipeline.GetInstance(ctx, statistics, gw.mod)
		if err != nil {
			logger.Errorf("[launch pipeline %s-#%d failed: %s]", newPipeline.Name(), i, err)
			return
		}

		pi := newPipelineInstance(instance)

		go pi.run()

		pipes := gw.pipelines[newPipeline.Name()]
		pipes = append(pipes, pi)
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

	pipeCtx, exists := gw.pipelineContexts[deletedPipeline.Name()]
	if !exists {
		logger.Errorf("[BUG: deleted pipeline %s have not context.]", deletedPipeline.Name())
		return
	}

	go pipeCtx.Close()

	delete(gw.pipelineContexts, deletedPipeline.Name())
}

func (gw *Gateway) loadPlugins() error {
	specs, err := gw.repo.GetAllPlugins()
	if err != nil {
		logger.Errorf("[load plugins from storage failed: %s]", err)
		return err
	}

	err = gw.mod.LoadPlugins(specs)
	if err != nil {
		logger.Errorf("[load model from plugin repository failed: %s]", err)
		return err
	}

	logger.Infof("[plugins are loaded successfully (total=%d)]", len(specs))

	return nil
}

func (gw *Gateway) loadPipelines() error {
	specs, err := gw.repo.GetAllPipelines()
	if err != nil {
		logger.Errorf("[load pipelines from storage failed: %s]", err)
		return err
	}

	err = gw.mod.LoadPipelines(specs)
	if err != nil {
		logger.Errorf("[load model fomr pipeline repository failed: %s]", err)
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

func (gw *Gateway) addPluginToStorage(newPlugin *model.Plugin) {
	spec := &config.PluginSpec{
		Type:   newPlugin.Type(),
		Config: newPlugin.Config(),
	}

	err := gw.repo.AddPlugin(spec)
	if err != nil {
		logger.Errorf("[add plugin %s failed: %s]", newPlugin.Name(), err)
	}
}

func (gw *Gateway) deletePluginFromStorage(deletedPlugin *model.Plugin) {
	err := gw.repo.DeletePlugin(deletedPlugin.Name())
	if err != nil {
		logger.Errorf("[delete plugin %s failed: %s]",
			deletedPlugin.Name(), err)
	}
}

func (gw *Gateway) updatePluginInStorage(updatedPlugin *model.Plugin) {
	spec := &config.PluginSpec{
		Type:   updatedPlugin.Type(),
		Config: updatedPlugin.Config(),
	}

	err := gw.repo.UpdatePlugin(spec)
	if err != nil {
		logger.Errorf("[update plugin %s failed: %s]", updatedPlugin.Name(), err)
	}
}

func (gw *Gateway) addPipelineToStorage(newPipeline *model.Pipeline) {
	spec := &config.PipelineSpec{
		Type:   newPipeline.Type(),
		Config: newPipeline.Config(),
	}
	err := gw.repo.AddPipeline(spec)
	if err != nil {
		logger.Errorf("[add pipeline %s failed: %s]", newPipeline.Name(), err)
	}
}

func (gw *Gateway) deletePipelineFromStorage(deletedPipeline *model.Pipeline) {
	err := gw.repo.DeletePipeline(deletedPipeline.Name())
	if err != nil {
		logger.Errorf("[delete pipeline %s failed: %s]", deletedPipeline.Name(), err)
	}
}

func (gw *Gateway) updatePipelineInStorage(updatedPipeline *model.Pipeline) {
	spec := &config.PipelineSpec{
		Type:   updatedPipeline.Type(),
		Config: updatedPipeline.Config(),
	}

	err := gw.repo.UpdatePipeline(spec)
	if err != nil {
		logger.Errorf("[update pipeline %s failed: %s]", updatedPipeline.Name(), err)
	}
}
