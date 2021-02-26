package jmx

import (
	"fmt"
	"testing"
)

func TestGetMbeanAttribute(t *testing.T) {

	client := NewClient("127.0.0.1", "8778", "jolokia")
	attribute, _ := client.GetMbeanAttribute("com.easeagent.jmx:type=SystemConfig", "HeapMemoryUsage", "")
	fmt.Println(attribute)


}
