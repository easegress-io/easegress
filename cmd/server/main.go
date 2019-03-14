package main

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/megaease/easegateway/pkg/option"

	"github.com/megaease/easegateway/pkg/api"
	"github.com/megaease/easegateway/pkg/cluster"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/model"
	"github.com/megaease/easegateway/pkg/profile"
	"github.com/megaease/easegateway/pkg/stat"
	"github.com/megaease/easegateway/pkg/store"
	"github.com/megaease/easegateway/pkg/version"
)

func main() {
	defer logger.Close()

	logger.Infof("%s", version.Long)

	profile, err := profile.New()
	if err != nil {
		logger.Errorf("new profile failed: %v", err)
		os.Exit(1)
	}

	cluster, err := cluster.NewCluster(*option.Global)
	if err != nil {
		logger.Errorf("new cluster failed: %v", err)
		os.Exit(1)
	}
	store, err := store.New(cluster)
	if err != nil {
		logger.Errorf("new store failed: %v", err)
		os.Exit(1)
	}
	model, err := model.NewModel(store)
	if err != nil {
		logger.Errorf("new model failed: %v", err)
		os.Exit(1)
	}

	stat := stat.NewStat(cluster, model)

	api := api.MustNewAPIServer(cluster)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	go func() {
		sig := <-sigChan
		logger.Infof("%s signal received, closing easegateway immediately", sig)
		os.Exit(255)
	}()

	logger.Infof("%s signal received, closing easegateway", sig)

	wg := &sync.WaitGroup{}
	wg.Add(6)
	api.Close(wg)
	stat.Close(wg)
	model.Close(wg)
	store.Close(wg)
	cluster.Close(wg)
	profile.Close(wg)
	wg.Wait()
}
