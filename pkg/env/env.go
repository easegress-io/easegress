/*
 * Copyright (c) 2017, The Easegress Authors
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package env provides functions for environment variables.
package env

import (
	"github.com/megaease/easegress/v2/pkg/common"
	"github.com/megaease/easegress/v2/pkg/option"
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

	if opt.AbsLogDir != "" {
		err = common.MkdirAll(opt.AbsLogDir)
		if err != nil {
			return err
		}
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
	if opt.AbsLogDir != "" {
		common.RemoveAll(opt.AbsLogDir)
	}
	common.RemoveAll(opt.AbsHomeDir)
}
