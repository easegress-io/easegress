/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/megaease/easegateway/pkg/api"
	"github.com/megaease/easegateway/pkg/cluster"
	"github.com/megaease/easegateway/pkg/common"
	"github.com/megaease/easegateway/pkg/env"
	"github.com/megaease/easegateway/pkg/graceupdate"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/option"
	"github.com/megaease/easegateway/pkg/pidfile"
	"github.com/megaease/easegateway/pkg/profile"
	"github.com/megaease/easegateway/pkg/supervisor"
	"github.com/megaease/easegateway/pkg/version"

	// For register stuff.
	_ "github.com/megaease/easegateway/pkg/registry"
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
		err := pidfile.Write(opt)
		if err != nil {
			logger.Errorf("write pidfile failed: %v", err)
			os.Exit(1)
		}
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
	super := supervisor.MustNew(opt, cls)
	supervisor.InitGlobalSupervisor(super)
	apiServer := api.MustNewServer(opt, cls)

	if graceupdate.CallOriProcessTerm(super.FirstHandleDone()) {
		pidfile.Write(opt)
	}

	closeCls := func() {
		wg := &sync.WaitGroup{}
		wg.Add(2)
		apiServer.Close(wg)
		cls.CloseServer(wg)
		wg.Wait()
	}
	restartCls := func() {
		cls.StartServer()
		apiServer = api.MustNewServer(opt, cls)
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
	apiServer.Close(wg)
	super.Close(wg)
	cls.Close(wg)
	profile.Close(wg)
	wg.Wait()
}
