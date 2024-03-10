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

package option

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/pflag"
)

// SetFlagsFromEnv sets flags from environment variables.
func SetFlagsFromEnv(prefix string, fs *pflag.FlagSet) error {
	var err error
	fs.VisitAll(func(f *pflag.Flag) {
		if err2 := setFlagFromEnv(fs, prefix, f.Name); err2 != nil {
			err = err2
		}
	})
	return err
}

func flagToEnv(prefix, name string) string {
	return prefix + "_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
}

func setFlagFromEnv(fs *pflag.FlagSet, prefix, flagName string) error {
	key := flagToEnv(prefix, flagName)
	val := os.Getenv(key)
	if val != "" {
		if err := fs.Set(flagName, val); err != nil {
			return fmt.Errorf("invalid value %q for %s: %v", val, key, err)
		}
	}
	return nil
}
