package env

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/option"
)

const (
	// backupRetains is backup retain number of member files.
	backupRetains int = 7
)

// Init initializes the sub directories. This does not use logger for errors since the logger dir not created yet.
func Init(opt *option.Options) error {
	err := os.MkdirAll(expandDir(opt.DataDir), 0750)
	if err != nil {
		return err
	}

	err = os.MkdirAll(expandDir(opt.ConfDir), 0750)
	if err != nil {
		return err
	}

	err = os.MkdirAll(expandDir(opt.LogDir), 0750)
	if err != nil {
		return err
	}

	go houseKeepMemberBackups(expandDir(opt.ConfDir))

	return nil
}

// expandDir checks `dir`, keep no change if it's absolute, otherwise prepend with the parent dir of this executable.
func expandDir(dir string) string {
	appDir := filepath.Dir(os.Args[0])
	if filepath.IsAbs(dir) {
		return filepath.Clean(dir)
	}
	return filepath.Clean(filepath.Join(appDir, dir))
}

func houseKeepMemberBackups(confDir string) {
	for {

		files, err := ioutil.ReadDir(confDir)
		if err != nil {
			logger.Errorf("Failed to list files under %s, error: %s", confDir, err)
			time.Sleep(5 * time.Minute)
			continue
		}

		var filenames = make([]string, 0)
		for _, f := range files {
			logger.Debugf("dir: %s, filename: %s", confDir, f.Name())
			matched, err := regexp.MatchString("^members\\.yaml.*bak$", f.Name())
			if err != nil {
				logger.Errorf("Failed to list backup files of members.yaml, error: %s", err)
			}

			if matched {
				filenames = append(filenames, f.Name())
			}
		}

		sort.Strings(filenames)

		for i := 0; i < len(filenames)-backupRetains; i++ {
			err := os.Remove(filepath.Join(confDir, filenames[i]))
			if err != nil {
				logger.Errorf("Failed to remove file %s, error: %v", filenames[i], err)
			}
		}

		time.Sleep(5 * time.Minute)
	}

}
