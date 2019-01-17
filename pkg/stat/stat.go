package stat

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/megaease/easegateway/pkg/logger"

	"github.com/megaease/easegateway/pkg/cluster"
	"github.com/megaease/easegateway/pkg/model"
)

type PipelineStat struct {
	Rate1  float64 `json:"rate1"`
	Rate5  float64 `json:"rate5"`
	Rate15 float64 `json:"rate15"`
}

// Stat is the middle level between cluster and model for statistics.
type Stat interface {
	Close()
}

type stat struct {
	cluster cluster.Cluster
	mod     *model.Model
	done    chan struct{}
}

func NewStat(cluster cluster.Cluster, mod *model.Model) Stat {
	s := &stat{
		cluster: cluster,
		mod:     mod,
		done:    make(chan struct{}),
	}

	go func() {
		for {
			select {
			case <-time.After(5 * time.Second):
				s.update()
			case <-s.done:
				return
			}
		}
	}()

	return s
}

func (s *stat) update() {
	kvs := make(map[string]*string)
	for name, stat := range s.mod.PipelineStats() {
		key := fmt.Sprintf(cluster.StatPipelineFormat, name)
		rate1, _ := stat.PipelineThroughputRate1()
		rate5, _ := stat.PipelineThroughputRate5()
		rate15, _ := stat.PipelineThroughputRate15()
		buff, _ := json.Marshal(PipelineStat{
			Rate1:  rate1,
			Rate5:  rate5,
			Rate15: rate15,
		})
		value := string(buff)
		kvs[key] = &value
	}

	if len(kvs) == 0 {
		return
	}

	err := s.cluster.PutAndDeleteUnderLease(kvs)
	if err != nil {
		logger.Errorf("update stat failed: %v", err)
	}
}

func (s *stat) Close() {
	close(s.done)
}
