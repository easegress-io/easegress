package sem

import (
	"math/rand"
	"sync"
	"testing"
	"time"
)

type Case struct {
	maxSem int
}

func TestSemaphore0(t *testing.T) {
	s := NewSem(0)

	c := make(chan struct{})
	go func(s *Semaphore, c chan struct{}) {
		s.Acquire()
		c <- struct{}{}
	}(s, c)

	select {
	case <-c:
		t.Errorf("trans: 1, maxSem: 0")
	case <-time.After(100 * time.Millisecond):
	}
}
func TestSemaphoreRobust(t *testing.T) {
	s := NewSem(10)
	w := &sync.WaitGroup{}
	w.Add(101)
	// change maxSem randomly
	go func() {
		for i := 0; i < 500; i++ {
			time.Sleep(1 * time.Millisecond)
			s.SetMaxCount(uint32(rand.Intn(100000)))
		}
		w.Done()
	}()

	for x := 0; x < 100; x++ {
		go func() {
			for j := 0; j < 100; j++ {
				s.Acquire()
				time.Sleep(5 * time.Millisecond)
				s.Release()
			}
			w.Done()
		}()
	}
	w.Wait()

	// confirm it still works
	s.SetMaxCount(20)
	time.Sleep(10 * time.Millisecond)

	runCase(s, &Case{23}, t)

}

func TestSemaphoreN(t *testing.T) {
	var s = NewSem(20)
	Cases := []*Case{
		{maxSem: 37},
		{maxSem: 45},
		{maxSem: 3},
		{maxSem: 1000},
		{maxSem: 235},
		{maxSem: 800},
		{maxSem: 587},
	}

	for _, c := range Cases {
		runCase(s, c, t)
	}
}

func BenchmarkSemaphore(b *testing.B) {
	s := NewSem(uint32(b.N/2 + 1))
	for i := 0; i < b.N; i++ {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				s.Acquire()
				s.Release()
			}
		})
	}
}
func runCase(s *Semaphore, c *Case, t *testing.T) {
	s.SetMaxCount(uint32(c.maxSem))
	time.Sleep(10 * time.Millisecond)

	w := &sync.WaitGroup{}
	w.Add(c.maxSem)
	for i := 0; i < c.maxSem; i++ {
		go func(w *sync.WaitGroup) {
			s.Acquire()
			defer s.Release()
			time.Sleep(100 * time.Millisecond)
			w.Done()
		}(w)
	}
	begin := time.Now()
	w.Wait()
	d := time.Since(begin)
	if d > 100*time.Millisecond+10*time.Millisecond {
		t.Errorf("time too long: %v, sem: %d, trans: %d", d, c.maxSem, c.maxSem)
	}
	if d < 100*time.Millisecond-10*time.Millisecond {
		t.Errorf("time too short: %v, sem: %d, trans: %d", d, c.maxSem, c.maxSem)
	}

	s.SetMaxCount(uint32(c.maxSem - 1))
	time.Sleep(10 * time.Millisecond)

	w = &sync.WaitGroup{}
	w.Add(c.maxSem)
	for i := 0; i < c.maxSem; i++ {
		go func(w *sync.WaitGroup) {
			s.Acquire()
			defer s.Release()
			time.Sleep(100 * time.Millisecond)
			w.Done()
		}(w)
	}
	begin = time.Now()
	w.Wait()
	d = time.Since(begin)
	if d < 200*time.Millisecond-10*time.Millisecond {
		t.Errorf("time too short: %v, sem: %d, trans: %d", d, c.maxSem-1, c.maxSem)
	}

	if d > 200*time.Millisecond+10*time.Millisecond {
		t.Errorf("time too long: %v, sem: %d, trans: %d", d, c.maxSem-1, c.maxSem)
	}
}
