package cluster

import (
	"time"

	yaml "gopkg.in/yaml.v2"

	"github.com/megaease/easegateway/pkg/logger"
)

const (
	// Keep alive hearbeat interval in seconds
	KEEP_ALIVE_INTERVAL = 5 * time.Second
)

type MemberStatus struct {
	// the name as an etcd member, only writer has a name
	Name string `yaml:name`
	// the id as an etcd member, only writer has an id
	Id uint64 `yaml:id`

	// writer | reader
	Role string `yaml:role`

	// leader | follower | subscriber | offline
	EtcdStatus string `yaml:etcd_status`

	// keepalive timestamp
	LastHeartbeatTime int64 `yaml:keepalive_time`
}

func etcd_heartbeat(c *cluster) {
	for {
		m := c.MemberStatus()
		buff, err := yaml.Marshal(m)
		if err != nil {
			logger.Errorf("Failed to fetch member status: error: %s", err)
			time.Sleep(KEEP_ALIVE_INTERVAL)
			continue
		}

		err = c.Put(MemberStatusPrefix+m.Name, string(buff))
		if err != nil {
			logger.Errorf("Failed to update member status: error: %s", err)
		}
		time.Sleep(KEEP_ALIVE_INTERVAL)
	}
}
