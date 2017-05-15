# Clustering Design of Ease Gateway

NOTICE: This is a underway design document to explain current cluster architecture under quick iteration, at some appropriate context there might be related knowledge or references to help new contributors jump in quickly.

## Background
In milestone 2, we wanna deploy Ease Gateway carrying with large scale use cases such as flash sale. So it's necessary to solve following clustering problems.
Here we put a regular issue we have now: In the scenario of flash sale, since there are different amounts of online users at every area waiting to rush to buy something, we must give different percents of pass for the crowd **at corresponding area**. It's hard to manually administrate so many instances carrying with different configure, thus we must design a general architecture for Ease Gateway to automate this kind of problems.

## Design (WIP)

### Clock Synchronization
The physical/wall clock can not fix global synchronization in distributed system. There have been already multiple ways to solve it. In Ease Gateway, there is no need to use complex vector clock, and then logical clock(lamport clock) satisfies the case enough. The original and complete source about lamport clock is the paper [Time, Clocks and the Ordering of Events in a Distributed System](http://lamport.azurewebsites.net/pubs/time-clocks.pdf).

Here we just introduce it in a simple way. Lamport clock is a mechanism for capturing chronological and causal relationships in a distributed system. According to [Wikipedia](https://en.wikipedia.org/wiki/Lamport_timestamps), it follows 3 simple rules:

> 1. A process increments its counter before each event in that process;
> 2. When a process sends a message, it includes its counter value with the message;
> 3. On receiving a message, the counter of the recipient is updated, if necessary, to the greater of its current counter and the timestamp in the received message. The counter is then incremented by 1 before the message is considered received.

We can implement it in golang simply:
```golang
package lamport

import (
	"sync/atomic"
)

// LamportClock is a thread safe implementation of a lamport clock. It
// uses efficient atomic operations for all of its functions, falling back
// to a heavy lock only if there are enough CAS failures.
type LamportClock struct {
	counter uint64
}

// LamportTime is the value of a LamportClock.
type LamportTime uint64

// Time is used to return the current value of the lamport clock
func (l *LamportClock) Time() LamportTime {
	return LamportTime(atomic.LoadUint64(&l.counter))
}

// Increment is used to increment and return the value of the lamport clock
func (l *LamportClock) Increment() LamportTime {
	return LamportTime(atomic.AddUint64(&l.counter, 1))
}

// Witness is called to update our local clock if necessary after
// witnessing a clock value received from another process
func (l *LamportClock) Witness(v LamportTime) {
	cur := atomic.LoadUint64(&l.counter)
	other := uint64(v)
	if other < cur {
		return
	}

	for !atomic.CompareAndSwapUint64(&l.counter, cur, other+1) {
	}
}
```

### Data Persistence
The persistence of cluster data is the same with data of standalone version like the configure of pipeline/plugin is stored in disk. But for some business data unnecessary for current node could be cleared under some condition.

### Serialization & Compression
JSON is friendly to debug. Binary protocols such as Protocol Buffers of Google, Thrift/Avro of Apache are good to bandwidth and efficiency. It's a tradeoff here.

As for compression, make a trade-off between CPU and bandwidth, it's appropriate to choose common gzip.

## API Specification (TODO)

## References
Sorted by occurrence time.

1. [Time, Clocks and the Ordering of Events in a Distributed System](http://lamport.azurewebsites.net/pubs/time-clocks.pdf)
2. [Wikipedia: Lamport Clock](https://en.wikipedia.org/wiki/Lamport_timestamps)
