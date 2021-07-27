// +build !windows

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

package common

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

var signalToOsMap = map[Signal]syscall.Signal{
	SignalInt:  syscall.SIGINT,
	SignalTerm: syscall.SIGTERM,
	SingalUsr2: syscall.SIGUSR2,
}

var signalFromOsMap = map[os.Signal]Signal{
	syscall.SIGINT:  SignalInt,
	syscall.SIGTERM: SignalTerm,
	syscall.SIGUSR2: SingalUsr2,
}

func NotifySignal(c chan<- Signal, sig ...Signal) error {
	if c == nil {
		return fmt.Errorf("SignalNotify using nil channel")
	}

	if len(sig) == 0 {
		return fmt.Errorf("SignalNotify must notify at least 1 signal")
	}

	ch := make(chan os.Signal, cap(c))

	sigs := make([]os.Signal, 0, len(sig))
	for _, s := range sig {
		oss, ok := signalToOsMap[s]
		if !ok {
			return fmt.Errorf("SignalNotify unsupported signal %v", s)
		}
		sigs = append(sigs, oss)
	}

	signal.Notify(ch, sigs...)

	go func() {
		for s := range ch {
			c <- signalFromOsMap[s]
		}
	}()

	return nil
}

func RaiseSignal(pid int, sig Signal) error {
	oss, ok := signalToOsMap[sig]

	if !ok {
		return fmt.Errorf("unsupported signal %v", sig)
	}

	return syscall.Kill(pid, oss)
}
