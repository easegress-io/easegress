package cluster

import (
	"crypto/md5"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"time"

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

	_, err := os.Stat(filename)
	if err == nil {
		bytes, err := ioutil.ReadFile(filename)
		if err != nil {
			return err
		}

		bakupFilename := filename + "." + time.Now().Format("2006-01-02T15:04:05.999") + ".bak"
		err = ioutil.WriteFile(bakupFilename, bytes, 0644)
		if err != nil {
			return err
		}
	}
	bytes, _ := yaml.Marshal(m)
	err = ioutil.WriteFile(filename+".tmp", bytes, 0644)
	if err != nil {
		return err
	}

	os.Rename(filename+".tmp", filename)

	return err

}

func (m *members) loadFromFile(filename string) error {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(bytes, m)
	return err
}

func (m *members) Len() int {
	return len(m.Members)
}

func (m *members) Swap(i, j int) {
	m.Members[i], m.Members[j] = m.Members[j], m.Members[i]
}

func (m *members) Less(i, j int) bool {
	if m.Members[i].Name != m.Members[j].Name {
		return m.Members[i].Name < m.Members[j].Name
	}

	return m.Members[i].PeerListener < m.Members[j].PeerListener

}

func (m *members) Md5() [16]byte {
	sort.Sort(m)
	str := strings.Builder{}
	for _, item := range m.Members {
		str.WriteString(item.Name)
		str.WriteString(item.PeerListener)
	}

	return md5.Sum([]byte(str.String()))
}
