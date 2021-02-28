package jmxtool

import (
	"fmt"
	"testing"
)

type heapMemoryUsage struct {
	committed int64
	init int64
	max int64
	used int64
}

func TestGetMbeanAttribute(t *testing.T) {

	client := NewJolokiaClient("127.0.0.1", "8778", "jolokia")

	// Read Value
	oldThreadCount, _ := client.SetMbeanAttribute("com.easeagent.jmx:type=SystemConfig", "ThreadCount", "", 11)
	fmt.Println(oldThreadCount)

	// Set value
	newThreadCount, _ := client.GetMbeanAttribute("com.easeagent.jmx:type=SystemConfig", "ThreadCount", "")
	fmt.Println(newThreadCount)


	newHeapMemoryUsage := heapMemoryUsage{
		init: 0,
		committed: 1234,
		max: 9999,
		used: 6666,
	}

	// Set value
	oldMemoryUsage, _ := client.SetMbeanAttribute("com.easeagent.jmx:type=SystemConfig", "HeapMemoryUsage", "", newHeapMemoryUsage)
	fmt.Println(oldMemoryUsage)

	// Read sub field of mbean
	newCommitted, _ := client.GetMbeanAttribute("com.easeagent.jmx:type=SystemConfig", "HeapMemoryUsage", "committed")
	fmt.Println(newCommitted)

	// Execute operation
	var args []interface{}
	args = append(args, 8)
	args = append(args, false)

	operation, err := client.ExecuteMbeanOperation("com.easeagent.jmx:type=SystemConfig", "doConfig", args)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(operation)

	// List mbean
	mbeanDetail, err := client.ListMbean("com.easeagent.jmx:type=SystemConfig")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(mbeanDetail)

	// Search mbeans
	mbeans, err := client.SearchMbeans("com.easeagent.jmx:*")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(mbeans)

}
