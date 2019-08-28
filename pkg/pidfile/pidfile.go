package pidfile

import (
	"os"
	"path/filepath"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/option"

	expidfile "github.com/megaease/pidfile"
)

const (
	pidfileName = "easegateway.pid"
)

var (
	pidfilePath string
)

// Write pidfile. if config pid-file is empty, skip it
func Write(opt *option.Options) error {
	pidfilePath = filepath.Join(opt.AbsHomeDir, pidfileName)

	expidfile.SetPidfilePath(pidfilePath)

	if err := expidfile.Write(); err != nil {
		logger.Errorf("pidfile write error: %s", err)
		return err
	}

	return nil
}

// Close cleans the pid file.
func Close() {
	if pidfilePath == "" {
		return
	}
	err := os.Remove(pidfilePath)
	if err != nil {
		logger.Errorf("remove %s failed: %v", pidfilePath, err)
	}
}
