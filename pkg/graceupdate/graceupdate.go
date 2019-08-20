package graceupdate

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/grace/gracenet"
)

var (
	// Global is gracenet Net struct
	Global     = &gracenet.Net{}
	didInherit = os.Getenv("LISTEN_FDS") != ""
	ppid       = os.Getppid()
)

// IsInherit : return true if it's update child process
func IsInherit() bool {
	return didInherit
}

// CallOriProcessTerm : When new server ready, notice ori process to exit
func CallOriProcessTerm(done chan struct{}) bool {
	if didInherit && ppid != 1 {
		<-done
		if err := syscall.Kill(ppid, syscall.SIGTERM); err != nil {
			logger.Errorf("failed to close parent: %s", err)
			return false
		}
		return true
	}
	return false
}

// NotifySigUsr2 : Set Signal usr2 notify
func NotifySigUsr2(closeCls func(), restartCls func()) {
	sigUsr2 := make(chan os.Signal, 1)
	signal.Notify(sigUsr2, syscall.SIGUSR2)
	go func() {
		sig := <-sigUsr2
		closeCls()
		logger.Infof("%s signal received, graceful update easegateway", sig)
		if pid, err := Global.StartProcess(); err != nil {
			logger.Errorf("graceful update failed: %v", err)
			restartCls()
			// Reset signal usr2 notify
			NotifySigUsr2(closeCls, restartCls)
		} else {
			childdone := make(chan error, 1)
			go func() {
				process, err := os.FindProcess(pid)
				if err != nil {
					restartCls()
					NotifySigUsr2(closeCls, restartCls)
				} else {
					_, werr := process.Wait()
					childdone <- werr
					select {
					case err := <-childdone:
						logger.Errorf("child proc exited: %v", err)
						restartCls()
						NotifySigUsr2(closeCls, restartCls)
					}
				}
			}()
		}
	}()
}
