/*
 * Copyright (c) 2017, The Easegress Authors
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

// Package graceupdate provides graceful update for easegress.
package graceupdate

import (
	"os"

	"github.com/megaease/grace/gracenet"

	"github.com/megaease/easegress/v2/pkg/common"
	"github.com/megaease/easegress/v2/pkg/logger"
)

var (
	// Global is gracenet Net struct
	Global     = &gracenet.Net{}
	didInherit = os.Getenv("LISTEN_FDS") != ""
	ppid       = os.Getppid()
)

// IsInherit returns if I am the child process
// on gracefully updating process.
func IsInherit() bool {
	return didInherit
}

// CallOriProcessTerm notifies parent process to exist.
func CallOriProcessTerm(done chan struct{}) bool {
	if didInherit && ppid != 1 {
		<-done
		if err := common.RaiseSignal(ppid, common.SignalTerm); err != nil {
			logger.Errorf("failed to close parent: %s", err)
			return false
		}
		return true
	}
	return false
}

// NotifySigUsr2 handles signal SIGUSR2 to gracefully update.
func NotifySigUsr2(closeCls func(), restartCls func()) error {
	sigUsr2 := make(chan common.Signal, 1)
	if err := common.NotifySignal(sigUsr2, common.SignalUsr2); err != nil {
		return err
	}

	// TODO: handle register error
	go func() {
		sig := <-sigUsr2
		closeCls()
		logger.Infof("%s signal received, graceful update easegress", sig)
		if pid, err := Global.StartProcess(); err != nil {
			logger.Errorf("graceful update failed: %v", err)
			restartCls()
			// Reset signal usr2 notify
			NotifySigUsr2(closeCls, restartCls)
		} else {
			go func() {
				process, err := os.FindProcess(pid)
				if err != nil {
					restartCls()
					NotifySigUsr2(closeCls, restartCls)
				} else {
					_, werr := process.Wait()
					logger.Errorf("child proc exited: %v", werr)
					restartCls()
					NotifySigUsr2(closeCls, restartCls)
				}
			}()
		}
	}()
	return nil
}
