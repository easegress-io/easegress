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

// IsInherit returns if I am the child process
// on gracefully updating process.
func IsInherit() bool {
	return didInherit
}

// CallOriProcessTerm notifies parent process to exist.
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

// NotifySigUsr2 handles signal SIGUSR2 to gracefaully update.
func NotifySigUsr2(closeCls func(), restartCls func()) {
	sigUsr2 := make(chan os.Signal, 1)
	signal.Notify(sigUsr2, syscall.SIGUSR2)
	go func() {
		sig := <-sigUsr2
		closeCls()
		logger.Infof("%s signal received, graceful update easegateway", sig)
		// logger.Infof("sleep for 5 seconds to wait etcd cluster carlmdown ...")
		// time.Sleep(5 * time.Second)
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
