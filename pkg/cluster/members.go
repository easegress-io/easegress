package cluster

import (
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

type members struct {
	Members []member `yaml:"nodes"`
}

type member struct {
	Name         string `yaml:"name"`
	PeerListener string `yaml:"peer_listener"`
}

const (
	KNOWN_MEMBERS_CFG_FILE = "members.yaml"
)

func newMembers() *members {
	members := new(members)
	members.Members = make([]member, 0, 1)

	return members
}

func (m *members) save2file(filename string) error {
	bytes, _ := yaml.Marshal(m)
	err := ioutil.WriteFile(filename, bytes, 0644)
	return err

}

func (m *members) loadFromfile(filename string) error {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(bytes, m)
	return err
}
