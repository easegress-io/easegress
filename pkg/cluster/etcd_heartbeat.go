package cluster

import (
	"time"

	yaml "gopkg.in/yaml.v2"

	"github.com/megaease/easegateway/pkg/logger"
)

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
