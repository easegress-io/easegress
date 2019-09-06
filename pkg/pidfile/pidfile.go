package pidfile

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/option"
)

const (
	pidfileName = "easegateway.pid"
)

var (
	pidfilePath string
)

// Write writes pidfile.
func Write(opt *option.Options) error {
	pidfilePath = filepath.Join(opt.AbsHomeDir, pidfileName)

	err := ioutil.WriteFile(pidfilePath, []byte(fmt.Sprintf("%d", os.Getpid())), 0644)
	if err != nil {
		logger.Errorf("write %s failed: %s", err)
		return err
	}

	return nil
}
