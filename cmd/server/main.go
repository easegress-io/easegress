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
	gracenet "github.com/megaease/easegateway/pkg/grace/net"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/option"
	"github.com/megaease/easegateway/pkg/profile"
	"github.com/megaease/easegateway/pkg/scheduler"
	"github.com/megaease/easegateway/pkg/version"
)

func main() {
	//flag for graceful update child proc
	didInherit := os.Getenv("EG_LISTEN_FDS") != ""
	ppid := os.Getppid()
	opt := option.New()

	msg, err := opt.Parse()
	if err != nil {
		common.Exit(1, err.Error())
	}
	if msg != "" {
		common.Exit(0, msg)
	}

	//graceful update ,disable force-new-cluster
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

	// send sigterm to ori proc
	if didInherit && ppid != 1 {
		<-sdl.FirstDone()
		if err := syscall.Kill(ppid, syscall.SIGTERM); err != nil {
			logger.Errorf("failed to close parent: %s", err)
		}
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR2)
	for {
		sig := <-sigChan
		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
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
			return
		case syscall.SIGUSR2:
			logger.Infof("%s signal received, graceful update easegateway", sig)
			//Stop ori proc api and etcd server
			wg := &sync.WaitGroup{}
			wg.Add(2)
			api.Close(wg)
			cls.CloseServer(wg)
			wg.Wait()
			Net := gracenet.Gnet
			if process, err := Net.StartProcess(); err != nil {
				logger.Errorf("graceful update failed: %v", err)
				//child proc err,restart api and etcd
				cls.StartServer()
				api = egapi.MustNewServer(opt, cls)
			} else {
				childdone := make(chan error, 1)
				go func() {
					_, error := process.Wait()
					childdone <- error
					select {
					case err := <-childdone:
						logger.Errorf("child proc exited: %v", err)
						//child proc exit before ori proc, restart api and etcd
						cls.StartServer()
						api = egapi.MustNewServer(opt, cls)
					}
				}()
			}
		}
	}

}
