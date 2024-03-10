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

package utils

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWithInterrupt(t *testing.T) {
	ctx, cancel := WithInterrupt(context.Background())
	defer cancel() // Clean up resources

	// Create a channel to receive a signal when the context is canceled
	interrupted := make(chan struct{})
	go func() {
		<-ctx.Done()
		close(interrupted)
	}()

	// Send an interrupt signal to the current process
	signal := syscall.SIGINT
	process, err := os.FindProcess(os.Getpid())
	assert.Nil(t, err)
	err = process.Signal(signal)
	assert.Nil(t, err)

	// Wait for context to be canceled or timeout
	select {
	case <-interrupted:
		// Context was canceled as expected
	case <-time.After(time.Second * 2):
		assert.Fail(t, "Context was not canceled within the expected time")
	}
}

func TestNewExecCmd(t *testing.T) {
	// create context and cancel function
	ctx, cancel := context.WithCancel(context.Background())

	cmd := NewExecCmd(ctx, "sleep", "10")

	assert.Equal(t, []string{"sleep", "10"}, cmd.Args)
	assert.Equal(t, os.Stdout, cmd.Stdout)
	assert.Equal(t, os.Stderr, cmd.Stderr)

	// start the process
	err := cmd.Start()
	assert.Nil(t, err)

	// cancel the context and check if the cancel function is called
	cancel()
	err = cmd.Wait()
	assert.NotNil(t, err)
}
