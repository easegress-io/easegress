package common

import (
	"fmt"
	"strings"
	"time"

	"github.com/rcrowley/go-metrics"
)

type PluginCommonConfig struct {
	Name string `json:"plugin_name"`
}

func (c *PluginCommonConfig) PluginName() string {
	return c.Name
}

func (c *PluginCommonConfig) Prepare(pipelineNames []string) error {
	c.Name = strings.TrimSpace(c.Name)
	if len(c.Name) == 0 {
		return fmt.Errorf("invalid plugin name")
	}

	return nil
}

///
type ThroughputStatisticType int

const (
	ThroughputRate1  ThroughputStatisticType = iota
	ThroughputRate5
	ThroughputRate15
)

type ThroughputStatistic struct {
	// EWMA is thread safe
	ewma metrics.EWMA
	done chan struct{}
}

func NewThroughputStatistic(typ ThroughputStatisticType) *ThroughputStatistic {
	var ewma metrics.EWMA
	switch typ {
	case ThroughputRate1:
		ewma = metrics.NewEWMA1()
	case ThroughputRate5:
		ewma = metrics.NewEWMA5()
	case ThroughputRate15:
		ewma = metrics.NewEWMA15()
	}

	done := make(chan struct{})
	tickFunc := func(ewma metrics.EWMA) {
		ticker := time.NewTicker(time.Duration(5) * time.Second)
		for {
			select {
			case <-ticker.C:
				// ewma.Tick ticks the clock to update the moving average.  It assumes it is called
				// every five seconds.
				ewma.Tick()
			case <-done:
				ticker.Stop()
				return
			}
		}
	}
	go tickFunc(ewma)
	return &ThroughputStatistic{
		ewma: ewma,
		done: done,
	}
}

func (t *ThroughputStatistic) Get() (float64, error) {
	return t.ewma.Rate(), nil
}

func (t *ThroughputStatistic) Update(n int64) {
	t.ewma.Update(n)
}

func (t *ThroughputStatistic) Close() error { // io.Closer stub
	close(t.done)
	return nil
}
