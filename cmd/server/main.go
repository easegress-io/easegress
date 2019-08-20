package main

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	egapi "github.com/megaease/easegateway/pkg/api"
	"github.com/megaease/easegateway/pkg/cluster"
	"github.com/megaease/easegateway/pkg/common"
	"github.com/megaease/easegateway/pkg/env"
	"github.com/megaease/easegateway/pkg/graceupdate"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/option"
	"github.com/megaease/easegateway/pkg/pidfile"
	"github.com/megaease/easegateway/pkg/profile"
	"github.com/megaease/easegateway/pkg/scheduler"
	"github.com/megaease/easegateway/pkg/version"
)

func main() {
	opt := option.New()
	msg, err := opt.Parse()
	if err != nil {
		common.Exit(1, err.Error())
	}
	if msg != "" {
		common.Exit(0, msg)
	}

	err = env.InitServerDir(opt)
	if err != nil {
		log.Printf("failed to init env: %v", err)
		os.Exit(1)
	}

	logger.Init(opt)
	defer logger.Sync()
	logger.Infof("%s", version.Long)

	// disable force-new-cluster for graceful update
	if graceupdate.IsInherit() {
		opt.ForceNewCluster = false
	} else {
		pidfile.Write(opt)
	}

	profile, err := profile.New(opt)
	if err != nil {
		logger.Errorf("new profile failed: %v", err)
		os.Exit(1)
	}
	cls, err := cluster.New(opt)
	if err != nil {
		logger.Errorf("new cluster failed: %v", err)
		os.Exit(1)
	}
	sdl := scheduler.MustNew(cls)
	api := egapi.MustNewServer(opt, cls)

	if graceupdate.CallOriProcessTerm(sdl.FirstDone()) {
		pidfile.Write(opt)
	}

	closeCls := func() {
		wg := &sync.WaitGroup{}
		wg.Add(2)
		api.Close(wg)
		cls.CloseServer(wg)
		wg.Wait()
	}
	restartCls := func() {
		cls.StartServer()
		api = egapi.MustNewServer(opt, cls)
	}
	graceupdate.NotifySigUsr2(closeCls, restartCls)

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
