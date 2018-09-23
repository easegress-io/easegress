package cli

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

type runtimeConfig struct {
	Sequences map[string]uint64 `json:"operation_seqs"`
}

type rcJSONFileStore struct {
	path string
}

func newRCJSONFileStore(path string) (*rcJSONFileStore, error) {
	if _, err := os.Stat(rcFullPath); os.IsNotExist(err) {
		file, err := os.OpenFile(rcFullPath, os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			return nil, err
		}
		file.WriteString("{}")
		file.Close()
	}

	return &rcJSONFileStore{
		path: path,
	}, nil
}

func (s *rcJSONFileStore) save(rc *runtimeConfig) error {
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

func (s *rcJSONFileStore) load() (*runtimeConfig, error) {
	buff, err := ioutil.ReadFile(s.path)
	if err != nil {
		return nil, fmt.Errorf("read config file %s failed: %v", s.path, err)
	}

	rc := new(runtimeConfig)
	err = json.Unmarshal(buff, rc)
	if err != nil {
		return nil, err
	}

	if rc.Sequences == nil {
		rc.Sequences = make(map[string]uint64)
	}

	return rc, nil
}
