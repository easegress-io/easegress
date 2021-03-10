package jmxtool

import (
	"fmt"
	"github.com/fatih/structs"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"testing"
)

type heapMemoryUsage struct {
	committed int64
	init      int64
	max       int64
	used      int64
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
		init:      0,
		committed: 1234,
		max:       9999,
		used:      6666,
	}

	// Set value
	oldMemoryUsage, _ := client.SetMbeanAttribute("com.easeagent.jmx:type=SystemConfig", "HeapMemoryUsage", "", newHeapMemoryUsage)
	fmt.Println(oldMemoryUsage)

	// Read sub field of mbean
	newCommitted, _ := client.GetMbeanAttribute("com.easeagent.jmx:type=SystemConfig", "HeapMemoryUsage", "committed")
	fmt.Println(newCommitted)

	// List mbean
	//mbeanDetail, err := client.ListMbean("com.easeagent.jmx:type=SystemConfig")
	//if err != nil {
	//	fmt.Println(err)
	//}
	//fmt.Println(mbeanDetail)
	//
	//// Search mbeans
	//mbeans, err := client.SearchMbeans("com.easeagent.jmx:*")
	//if err != nil {
	//	fmt.Println(err)
	//}
	//fmt.Println(mbeans)

}

func TestExecuteMbeanOperation(t *testing.T) {
	// Execute operation
	client := NewJolokiaClient("127.0.0.1", "8778", "jolokia")

	observability := spec.Observability{}
	observability.OutputServer = &spec.ObservabilityOutputServer{
		Enabled:         true,
		BootstrapServer: "128.0.0.1",
	}

	observabilityTracingDetail := spec.ObservabilityTracingDetail{
		Enabled:       true,
		ServicePrefix: "agent",
	}

	observability.Tracing = &spec.ObservabilityTracing{
		Topic:        "KAFKA",
		SampledByQPS: 123,
		Request:      observabilityTracingDetail,
		RemoteInvoke: observabilityTracingDetail,
		Kafka:        observabilityTracingDetail,
		Jdbc:         observabilityTracingDetail,
		Redis:        observabilityTracingDetail,
		Rabbit:       observabilityTracingDetail,
	}

	observabilityMetricDetail := spec.ObservabilityMetricDetail{
		Enabled:  false,
		Interval: 1,
		Topic:    "aaa",
	}
	observability.Metric = &spec.ObservabilityMetric{
		Request:        observabilityMetricDetail,
		JdbcConnection: observabilityMetricDetail,
		JdbcStatement:  observabilityMetricDetail,
		Rabbit:         observabilityMetricDetail,
		Redis:          observabilityMetricDetail,
		Kafka:          observabilityMetricDetail,
	}

	m := structs.Map(observability)

	args := []interface{}{m}
	operation, err := client.ExecuteMbeanOperation("com.easeagent.jmx:type=SystemConfig", "updateConfigs", args)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(operation)
}
