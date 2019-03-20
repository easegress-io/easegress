package environ

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

// InitDirs initialize the sub directories. This does not use logger for errors since the logger dir not created yet.
func InitDirs(opt option.Options) error {
	err := os.MkdirAll(ExpandDir(opt.DataDir), 0750)
	if err != nil {
		return err
	}

	err = os.MkdirAll(ExpandDir(opt.ConfDir), 0750)
	if err != nil {
		return err
	}

	err = os.MkdirAll(ExpandDir(opt.LogDir), 0750)
	if err != nil {
		return err
	}

	return nil
}

// ExpandDir checks `dir`, keep no change if it's absolute, otherwise prepend with the parent dir of this executable.
func ExpandDir(dir string) string {
	appDir := filepath.Dir(os.Args[0])
	if filepath.IsAbs(dir) {
		return filepath.Clean(dir)
	} else {
		return filepath.Clean(filepath.Join(appDir, dir))
	}
}

func HouseKeepMemberBackups(confDir string) {
	const BACKUP_RETAINS int = 7

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

		for i := 0; i < len(filenames)-BACKUP_RETAINS; i++ {
			err := os.Remove(filepath.Join(confDir, filenames[i]))
			if err != nil {
				logger.Errorf("Failed to remove file %s, error: %v", filenames[i], err)
			}
		}

		time.Sleep(5 * time.Minute)
	}

}
