package common

import (
	"testing"
	"time"
)

func BenchmarkTimeNow(b *testing.B) {
	for i := 0; i < b.N; i++ {
		time.Now()
	}
}

func BenchmarkNowGettimeofday(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Now()
	}
}

func BenchmarkNowUnixNanoGettimeofday(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NowUnixNano()
	}
}
