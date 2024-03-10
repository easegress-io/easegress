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

// Package resources provides the resources utilities for the client.
package resources

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/megaease/easegress/v2/cmd/client/general"
)

const DefaultNamespace = "default"

// GetResourceKind returns the kind of the resource.
func GetResourceKind(arg string) (string, error) {
	if general.InAPIResource(arg, CustomData()) {
		return CustomData().Kind, nil
	}
	if general.InAPIResource(arg, CustomDataKind()) {
		return CustomDataKind().Kind, nil
	}
	if general.InAPIResource(arg, Member()) {
		return Member().Kind, nil
	}
	objects, err := ObjectAPIResources()
	if err != nil {
		return "", err
	}
	for _, object := range objects {
		if general.InAPIResource(arg, object) {
			return object.Kind, nil
		}
	}
	return "", fmt.Errorf("unknown resource: %s", arg)
}

func getResourceTempFilePath(kind, name string) string {
	ts := time.Now().Format("2006-01-02-1504")
	return filepath.Join(os.TempDir(), fmt.Sprintf("%s-%s-%s.yaml", kind, name, ts))
}

func execEditor(filePath string) error {
	editor := os.Getenv("EGCTL_EDITOR")
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.Command(editor, filePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func editErrWithPath(err error, filePath string) error {
	return fmt.Errorf("%s\n  yaml saved in %s", err.Error(), filePath)
}

func editResource(oldYaml, filePath string) (string, error) {
	var err error
	if err = os.WriteFile(filePath, []byte(oldYaml), 0644); err != nil {
		return "", err
	}
	// exec editor and get new yaml
	if err = execEditor(filePath); err != nil {
		return "", err
	}

	var newYaml []byte
	if newYaml, err = os.ReadFile(filePath); err != nil {
		return "", err
	}
	if string(newYaml) == oldYaml {
		fmt.Printf("nothing changed; temp file in %s\n", filePath)
		return "", nil
	}
	return string(newYaml), nil
}
