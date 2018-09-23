// +build darwin dragonfly freebsd netbsd openbsd solaris windows plan9

package common

import (
	"syscall"
)

func SysProcAttr() *syscall.SysProcAttr {
	return nil
}
