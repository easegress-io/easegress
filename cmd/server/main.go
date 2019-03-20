package main

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/megaease/easegateway/cmd/server/environ"
	"github.com/megaease/easegateway/pkg/api"
	"github.com/megaease/easegateway/pkg/cluster"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/option"
	"github.com/megaease/easegateway/pkg/profile"
	"github.com/megaease/easegateway/pkg/scheduler"
	"github.com/megaease/easegateway/pkg/version"
)

func main() {
	defer logger.Close()

	logger.Infof("%s", version.Long)

	err := environ.InitDirs(*option.Global)
	if err != nil {
		logger.Errorf("Failed to create directories, error: %s", err)
		os.Exit(1)
	}
	go environ.HouseKeepMemberBackups(environ.ExpandDir(option.Global.ConfDir))

	profile, err := profile.New()
	if err != nil {
		logger.Errorf("new profile failed: %v", err)
		os.Exit(1)
	}

	cls, _, err := cluster.New(*option.Global)
	if err != nil {
		logger.Errorf("new cluster failed: %v", err)
		os.Exit(1)
	}

	sdl, err := scheduler.New(cls)
	if err != nil {
		logger.Errorf("new scheduler failed: %v", err)
	}

	api := api.MustNewServer(cls)

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
	wg.Add(4)
	api.Close(wg)
	sdl.Close(wg)
	cls.Close(wg)
	profile.Close(wg)
	wg.Wait()
}
