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

package grpcserver

import (
	"sync"
	"testing"
	"time"

	"github.com/megaease/easegress/v2/pkg/context/contexttest"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/stretchr/testify/assert"
)

func TestNormal(t *testing.T) {
	at := assert.New(t)
	s := `
kind: GRPCServer
maxConnections: 1024
maxConnectionIdle: 60s
port: 8850
name: server-grpc
minTime: 1s
`
	spec, err := supervisor.NewSpec(s)
	at.NoError(err)

	r := newRuntime(spec, &contexttest.MockedMuxMapper{})
	defer func() {
		if r.getState() == stateRunning {
			done := make(chan struct{})
			r.handleEventClose(&eventClose{done: done})
			<-done
		}
	}()
	at.Equal(stateNil, r.getState())
	r.spec = nil
	r.reload(spec, &contexttest.MockedMuxMapper{})
	at.Equal(stateRunning, r.getState())
	// to ensure r.server started
	time.Sleep(200 * time.Millisecond)
	r.Close()
	at.Equal(stateClosed, r.getState())
	// follow test to ensure go routine func checkFailed in runtime will finish
	time.Sleep(checkFailedTimeout + 1*time.Second)
	r.setState(stateFailed)
	flag := false
	c := make(chan struct{})
	wait := &sync.WaitGroup{}
	wait.Add(1)
	go func() {
		wait.Done()
		select {
		case <-r.eventChan:
		case <-c:
			return
		}
		flag = true
	}()
	wait.Wait()
	time.Sleep(200 * time.Millisecond)
	// wait func checkFailed finish

	at.False(flag)
	c <- struct{}{}
}

func TestBuildServerKeepaliveOpt(t *testing.T) {
	r := &runtime{
		spec: &Spec{
			MinTime:               "1s",
			MaxConnectionAge:      "10s",
			MaxConnectionIdle:     "60s",
			MaxConnectionAgeGrace: "10s",
			Time:                  "3s",
			Timeout:               "4s",
		},
	}
	opts := r.buildServerKeepaliveOpt()
	assert.Equal(t, 2, len(opts))
}
