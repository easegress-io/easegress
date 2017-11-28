package common

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/codahale/hdrhistogram"
)

type HDRSample struct {
	bucketIdx     uint64
	bucketIdxLock sync.RWMutex
	buckets       []*hdrhistogram.Histogram
	bucketLocks   []sync.RWMutex
	stop          chan struct{}
	closeLock     sync.RWMutex
	closed        bool
}

func NewHDRSample(timeRange time.Duration, secondsForEachBucket int) *HDRSample {
	if secondsForEachBucket == 0 {
		secondsForEachBucket = int(timeRange / 2)
	}

	s := new(HDRSample)

	n := uint64(math.Ceil(timeRange.Seconds()/float64(secondsForEachBucket))) + 1
	s.buckets = make([]*hdrhistogram.Histogram, n)

	var idx uint64 = 0
	for idx < n {
		// FIXME(zhiyan): decrease min value or increase max value if the data range of sample value is not proper
		s.buckets[idx] = hdrhistogram.New(0, int64(30*time.Minute), 3)
		idx++
	}

	s.bucketLocks = make([]sync.RWMutex, n)

	switcher := func() {
		for {
			select {
			case <-time.Tick(time.Second * time.Duration(secondsForEachBucket)):
				s.bucketIdxLock.Lock()

				s.bucketLocks[s.bucketIdx].Lock()
				s.buckets[s.bucketIdx].Reset()
				s.bucketLocks[s.bucketIdx].Unlock()

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

func (s *HDRSample) Close() {
	s.closeLock.Lock()
	defer s.closeLock.Unlock()

	if s.closed {
		return
	}

	close(s.stop)
	s.closed = true
}

func (s *HDRSample) Update(v int64) error {
	for idx, bucket := range s.buckets {
		s.bucketLocks[idx].Lock()
		err := bucket.RecordValue(v)
		s.bucketLocks[idx].Unlock()

		if err != nil {
			fmt.Printf("update sample value failed, you might to change min/max value of HDR @developer: %v", err)
			return err
		}
	}

	return nil
}

func (s *HDRSample) Percentile(q float64) int64 {
	s.bucketIdxLock.RLock()
	defer s.bucketIdxLock.RUnlock()

	idx := (s.bucketIdx + 1) % uint64(len(s.buckets))

	s.bucketLocks[idx].RLock()
	defer s.bucketLocks[idx].RUnlock()

	return s.buckets[idx].ValueAtQuantile(q * 100)
}

func (s *HDRSample) StdDev() float64 {
	s.bucketIdxLock.RLock()
	defer s.bucketIdxLock.RUnlock()

	idx := (s.bucketIdx + 1) % uint64(len(s.buckets))

	s.bucketLocks[idx].RLock()
	defer s.bucketLocks[idx].RUnlock()

	return s.buckets[idx].StdDev()
}

func (s *HDRSample) Max() int64 {
	s.bucketIdxLock.RLock()
	defer s.bucketIdxLock.RUnlock()

	idx := (s.bucketIdx + 1) % uint64(len(s.buckets))

	s.bucketLocks[idx].RLock()
	defer s.bucketLocks[idx].RUnlock()

	return s.buckets[idx].Max()
}

func (s *HDRSample) Min() int64 {
	s.bucketIdxLock.RLock()
	defer s.bucketIdxLock.RUnlock()

	idx := (s.bucketIdx + 1) % uint64(len(s.buckets))

	s.bucketLocks[idx].RLock()
	defer s.bucketLocks[idx].RUnlock()

	return s.buckets[idx].Min()
}

func (s *HDRSample) Count() int64 {
	s.bucketIdxLock.RLock()
	defer s.bucketIdxLock.RUnlock()

	idx := (s.bucketIdx + 1) % uint64(len(s.buckets))

	s.bucketLocks[idx].RLock()
	defer s.bucketLocks[idx].RUnlock()

	return s.buckets[idx].TotalCount()
}
