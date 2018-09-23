// +build linux

package common

import (
	"syscall"
)

func SysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWNS | syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWPID | syscall.CLONE_NEWUTS | syscall.CLONE_NEWUSER,
	}
}
