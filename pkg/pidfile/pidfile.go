package pidfile

import (
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/option"
	expidfile "github.com/megaease/pidfile"
)

// Write pidfile. if config pid-file is empty, skip it
func Write(opt *option.Options) error {
	var pidfile = opt.PidFile
	if pidfile == "" {
		logger.Infof("pid-file is empty or not set, skip write")
		return nil
	}

	expidfile.SetPidfilePath(pidfile)

	if err := expidfile.Write(); err != nil {
		logger.Errorf("pidfile write error: %s", err)
		return err
	}
	return nil
}
