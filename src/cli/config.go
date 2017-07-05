package cli

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

type RCJSONFileStore struct {
	path string
}

func newRCJSONFileStore(path string) *RCJSONFileStore {
	return &RCJSONFileStore{
		path: path,
	}
}

func (s *RCJSONFileStore) save(rc *runtimeConfig) error {
	buff, err := json.Marshal(rc)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(s.path, buff, 0600)
	if err != nil {
		return err
	}
	return nil
}

func (s *RCJSONFileStore) load() (*runtimeConfig, error) {
	buff, err := ioutil.ReadFile(s.path)
	if err != nil {
		return nil, fmt.Errorf("read file %s failed: %v", s.path, err)
	}

	rc := new(runtimeConfig)
	err = json.Unmarshal(buff, rc)
	if err != nil {
		return nil, err
	}

	return rc, nil
}
