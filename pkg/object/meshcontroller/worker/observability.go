package worker

import (
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegateway/pkg/util/jmxtool"
	"strconv"
)

const (
	easeAgentConfigManager = "com.megaease.easeagent:type=ConfigManager"
)

type ObservabilityManager struct {
	serviceName   string
	jolokiaClient *jmxtool.JolokiaClient
}

func NewObservabilityServer(serviceName string) *ObservabilityManager {
	client := jmxtool.NewJolokiaClient("localhost", "8778", "jolokia")
	return &ObservabilityManager{
		serviceName:   serviceName,
		jolokiaClient: client,
	}
}

func (server *ObservabilityManager) UpdateObservability(serviceName string, newObservability *spec.Observability) error {

	paramsMap := server.getObservabilityParams(newObservability)
	args := []interface{}{paramsMap}
	result, err := server.jolokiaClient.ExecuteMbeanOperation(easeAgentConfigManager, "updateConfigs", args)
	if err != nil {
		logger.Errorf("UpdateObservability service :%s observability failed, Observability : %v , Result: %v,err : %v", serviceName, newObservability, result, err)
		return err
	}
	return nil
}

func (server *ObservabilityManager) getObservabilityParams(o *spec.Observability) *map[string]string {
	m := make(map[string]string)

	m["outputserver.Enabled"] = strconv.FormatBool(o.OutputServer.Enabled)
	m["outputserver.BootstrapServer"] = o.OutputServer.BootstrapServer

	m["tracing.Topic"] = o.Tracing.Topic
	m["tracing.SampledByQPS"] = strconv.Itoa(o.Tracing.SampledByQPS)
	m["tracing.Rabbit.ServicePrefix"] = o.Tracing.Rabbit.ServicePrefix
	m["tracing.Kafka.ServicePrefix"] = o.Tracing.Kafka.ServicePrefix
	m["tracing.Jdbc.ServicePrefix"] = o.Tracing.Jdbc.ServicePrefix
	m["tracing.Request.ServicePrefix"] = o.Tracing.Request.ServicePrefix
	m["tracing.Redis.ServicePrefix"] = o.Tracing.Redis.ServicePrefix
	m["tracing.RemoteInvoke.ServicePrefix"] = o.Tracing.RemoteInvoke.ServicePrefix

	m["metric.Request.Enabled"] = strconv.FormatBool(o.Metric.Request.Enabled)
	m["metric.Request.Interval"] = strconv.Itoa(o.Metric.Request.Interval)
	m["metric.Request.Topic"] = o.Metric.Request.Topic

	m["metric.Redis.Enabled"] = strconv.FormatBool(o.Metric.Redis.Enabled)
	m["metric.Redis.Interval"] = strconv.Itoa(o.Metric.Redis.Interval)
	m["metric.Redis.Topic"] = o.Metric.Redis.Topic

	m["metric.Rabbit.Enabled"] = strconv.FormatBool(o.Metric.Rabbit.Enabled)
	m["metric.Rabbit.Interval"] = strconv.Itoa(o.Metric.Rabbit.Interval)
	m["metric.Rabbit.Topic"] = o.Metric.Rabbit.Topic

	m["metric.Kafka.Enabled"] = strconv.FormatBool(o.Metric.Kafka.Enabled)
	m["metric.Kafka.Interval"] = strconv.Itoa(o.Metric.Kafka.Interval)
	m["metric.Kafka.Topic"] = o.Metric.Kafka.Topic

	m["metric.JdbcStatement.Enabled"] = strconv.FormatBool(o.Metric.JdbcStatement.Enabled)
	m["metric.JdbcStatement.Interval"] = strconv.Itoa(o.Metric.JdbcStatement.Interval)
	m["metric.JdbcStatement.Topic"] = o.Metric.JdbcStatement.Topic

	m["metric.JdbcConnection.Enabled"] = strconv.FormatBool(o.Metric.JdbcConnection.Enabled)
	m["metric.JdbcConnection.Interval"] = strconv.Itoa(o.Metric.JdbcConnection.Interval)
	m["metric.JdbcConnection.Topic"] = o.Metric.JdbcConnection.Topic

	return &m

}
