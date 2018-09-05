package common

import (
	"math"
	"sync"
	"time"

	"github.com/rcrowley/go-metrics"
)

type ExpDecaySample struct {
	bucketIdxLock sync.RWMutex
	bucketIdx     uint64
	buckets       []metrics.Sample
	stop          chan struct{}
	closeLock     sync.RWMutex
	closed        bool
}

func NewExpDecaySample(timeRange time.Duration, secondsForEachBucket int) *ExpDecaySample {
	if secondsForEachBucket == 0 {
		secondsForEachBucket = int(timeRange / 2)
	}

	s := new(ExpDecaySample)

	n := uint64(math.Ceil(timeRange.Seconds()/float64(secondsForEachBucket))) + 1

	var idx uint64 = 0
	for idx < n {
		s.buckets = append(s.buckets, metrics.NewExpDecaySample(514, 0.015))
		idx++
	}

	s.stop = make(chan struct{})
	switcher := func() {
		ticker := time.NewTicker(time.Second * time.Duration(secondsForEachBucket))
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.bucketIdxLock.Lock()

				s.buckets[s.bucketIdx].Clear()
				s.bucketIdx = (s.bucketIdx + 1) % n

				s.bucketIdxLock.Unlock()
			case <-s.stop:
				return
			}
		}
	}

	go switcher()

	return s
}

func (s *ExpDecaySample) Close() {
	s.closeLock.Lock()
	defer s.closeLock.Unlock()

	if s.closed {
		return
	}

	close(s.stop)
	s.closed = true

	// defense, accelerate gc
	for i := 0; i < len(s.buckets); i++ {
		s.buckets[i].Clear()
	}
}

func (s *ExpDecaySample) Update(v int64) {
	for i := 0; i < len(s.buckets); i++ {
		s.buckets[i].Update(v)
	}
}

func (s *ExpDecaySample) Percentile(q float64) float64 {
	s.bucketIdxLock.RLock()
	defer s.bucketIdxLock.RUnlock()

	idx := (s.bucketIdx + 1) % uint64(len(s.buckets))

	return s.buckets[idx].Percentile(q)
}

func (s *ExpDecaySample) StdDev() float64 {
	s.bucketIdxLock.RLock()
	defer s.bucketIdxLock.RUnlock()

	idx := (s.bucketIdx + 1) % uint64(len(s.buckets))

	return s.buckets[idx].StdDev()
}

func (s *ExpDecaySample) Max() int64 {
	s.bucketIdxLock.RLock()
	defer s.bucketIdxLock.RUnlock()

	idx := (s.bucketIdx + 1) % uint64(len(s.buckets))

	return s.buckets[idx].Max()
}

func (s *ExpDecaySample) Min() int64 {
	s.bucketIdxLock.RLock()
	defer s.bucketIdxLock.RUnlock()

	idx := (s.bucketIdx + 1) % uint64(len(s.buckets))

	return s.buckets[idx].Min()
}

func (s *ExpDecaySample) Count() int64 {
	s.bucketIdxLock.RLock()
	defer s.bucketIdxLock.RUnlock()

	idx := (s.bucketIdx + 1) % uint64(len(s.buckets))

	return s.buckets[idx].Count()
}

func (s *ExpDecaySample) Variance() float64 {
	s.bucketIdxLock.RLock()
	defer s.bucketIdxLock.RUnlock()

	idx := (s.bucketIdx + 1) % uint64(len(s.buckets))

	return s.buckets[idx].Variance()
}

func (s *ExpDecaySample) Sum() int64 {
	s.bucketIdxLock.RLock()
	defer s.bucketIdxLock.RUnlock()

	idx := (s.bucketIdx + 1) % uint64(len(s.buckets))

	return s.buckets[idx].Sum()
}
