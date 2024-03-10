//go:build !windows
// +build !windows

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

package common

import (
	"syscall"
	"testing"
)

func TestNotifySignalAndRaiseSignal(t *testing.T) {
	c := make(chan Signal, 5)
	var currSig Signal
	sigs := []Signal{SignalInt, SignalTerm, SignalUsr2}

	// NotifySignal
	if err := NotifySignal(nil); err == nil {
		t.Errorf("error for nil chan1")
	}
	if err := NotifySignal(nil, SignalInt, SignalTerm); err == nil {
		t.Errorf("error for nil chan2")
	}
	if err := NotifySignal(c); err == nil {
		t.Errorf("error for empty signal")
	}
	if err := NotifySignal(c, SignalInt, "BadSignal", SignalTerm); err == nil {
		t.Errorf("error for unsupported signal")
	}

	if err := NotifySignal(c, sigs...); err != nil {
		t.Fatalf("error with input %+v", sigs)
	}
	for _, s := range sigs {
		syscall.Kill(syscall.Getpid(), signalToOsMap[s])
		currSig = <-c
		if currSig != s {
			t.Fatalf("error signal from chan")
		}
	}

	// RaiseSignal
	if err := RaiseSignal(syscall.Getpid(), "BadSignal"); err == nil {
		t.Errorf("error with wrong input")
	}
	for _, s := range sigs {
		if err := RaiseSignal(syscall.Getpid(), s); err != nil {
			t.Fatalf("error raise signal")
		}
		currSig = <-c
		if currSig != s {
			t.Fatalf("error signal from chan")
		}
	}
}
