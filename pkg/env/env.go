package env

import (
	"github.com/megaease/easegateway/pkg/common"
	"github.com/megaease/easegateway/pkg/option"
)

// InitServerDir initializes subdirs server needs.
// This does not use logger for errors since the logger dir not created yet.
func InitServerDir(opt *option.Options) error {
	err := common.MkdirAll(opt.AbsHomeDir)
	if err != nil {
		return err
	}

	err = common.MkdirAll(opt.AbsDataDir)
	if err != nil {
		return err
	}

	if opt.AbsWALDir != "" {
		err = common.MkdirAll(opt.AbsWALDir)
		if err != nil {
			return err
		}
	}

	err = common.MkdirAll(opt.AbsLogDir)
	if err != nil {
		return err
	}

	err = common.MkdirAll(opt.AbsMemberDir)
	if err != nil {
		return err
	}

	return nil
}

// CleanServerDir cleans subdirs InitServerDir creates.
// It is mostly used for test functions.
func CleanServerDir(opt *option.Options) {
	common.RemoveAll(opt.AbsDataDir)
	if opt.AbsWALDir != "" {
		common.RemoveAll(opt.AbsWALDir)
	}
	common.RemoveAll(opt.AbsMemberDir)
	common.RemoveAll(opt.AbsLogDir)
	common.RemoveAll(opt.AbsHomeDir)
}
