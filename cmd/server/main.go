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
	"github.com/megaease/easegateway/pkg/profile"
	"github.com/megaease/easegateway/pkg/scheduler"
	"github.com/megaease/easegateway/pkg/version"
)

func main() {
	// LISTEN_FDS is flag for graceful update child proc
	didInherit := os.Getenv("LISTEN_FDS") != ""
	ppid := os.Getppid()
	opt := option.New()

	msg, err := opt.Parse()
	if err != nil {
		common.Exit(1, err.Error())
	}
	if msg != "" {
		common.Exit(0, msg)
	}

	// disable force-new-cluster for graceful update
	if didInherit {
		opt.ForceNewCluster = false
	}
	err = env.InitServerDir(opt)
	if err != nil {
		log.Printf("failed to init env: %v", err)
		os.Exit(1)
	}

	logger.Init(opt)
	defer logger.Sync()
	logger.Infof("%s", version.Long)

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

	if didInherit && ppid != 1 {
		<-sdl.FirstDone()
		if err := syscall.Kill(ppid, syscall.SIGTERM); err != nil {
			logger.Errorf("failed to close parent: %s", err)
		}
	}

	sigUsr2 := make(chan os.Signal, 1)
	signal.Notify(sigUsr2, syscall.SIGUSR2)
	go func() {
		sig := <-sigUsr2
		logger.Infof("%s signal received, graceful update easegateway", sig)
		wg := &sync.WaitGroup{}
		wg.Add(2)
		api.Close(wg)
		cls.CloseServer(wg)
		wg.Wait()
		gnet := graceupdate.Global
		if pid, err := gnet.StartProcess(); err != nil {
			logger.Errorf("graceful update failed: %v", err)
			cls.StartServer()
			api = egapi.MustNewServer(opt, cls)
		} else {
			childdone := make(chan error, 1)
			go func() {
				process, err := os.FindProcess(pid)
				if err != nil {
					cls.StartServer()
					api = egapi.MustNewServer(opt, cls)
				} else {
					_, werr := process.Wait()
					childdone <- werr
					select {
					case err := <-childdone:
						logger.Errorf("child proc exited: %v", err)
						cls.StartServer()
						api = egapi.MustNewServer(opt, cls)
					}
				}
			}()
		}
	}()
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
