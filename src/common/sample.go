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

func NewHDRSample(timeRange time.Duration, secondsForEachBucket int) *ExpDecaySample {
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
		for {
			select {
			case <-time.Tick(time.Second * time.Duration(secondsForEachBucket)):
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
}

func (s *ExpDecaySample) Update(v int64) {
	for _, bucket := range s.buckets {
		bucket.Update(v)
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
