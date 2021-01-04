package easemonitormetrics

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/object/httpserver"
	"github.com/megaease/easegateway/pkg/option"
	"github.com/megaease/easegateway/pkg/plugin/backend"
	"github.com/megaease/easegateway/pkg/scheduler"
	"github.com/megaease/easegateway/pkg/util/httpstat"

	"github.com/Shopify/sarama"
	jsoniter "github.com/json-iterator/go"
)

const (
	// Kind is EaseMonitorMetrics kind.
	Kind = "EaseMonitorMetrics"
)

var (
	hostIPv4 string
)

func init() {
	scheduler.Register(&scheduler.ObjectRecord{
		Kind:              Kind,
		DefaultSpecFunc:   DefaultSpec,
		NewFunc:           New,
		DependObjectKinds: nil,
	})

	hostIPv4 = getHostIPv4()
	if hostIPv4 == "" {
		panic(fmt.Errorf("get host ipv4 failed"))
	}
}

type (
	// EaseMonitorMetrics is Object EaseMonitorMetrics.
	EaseMonitorMetrics struct {
		spec *Spec

		// sarama.AsyncProducer
		client      atomic.Value
		clientMutex sync.Mutex

		latestTimestamp int64

		done chan struct{}
	}

	// Spec describes the EaseMonitorMetrics.
	Spec struct {
		scheduler.ObjectMeta `yaml:",inline"`

		Kafka *KafkaSpec `yaml:"kafka" jsonschema:"required"`
	}

	// KafkaSpec is the spec for kafka producer.
	KafkaSpec struct {
		Brokers []string `yaml:"brokers" jsonschema:"required,uniqueItems=true"`
		Topic   string   `yaml:"topic" jsonschema:"required"`
	}

	// Status is the status of EaseMonitorMetrics.
	Status struct {
		Timestamp int64  `json:"timestamp"`
		Health    string `json:"health"`
	}

	// GlobalFields is the global fieilds of EaseMonitor metrics.
	GlobalFields struct {
		Timestamp int64  `json:"timestamp"`
		Category  string `json:"category"`
		HostName  string `json:"host_name"`
		HostIpv4  string `json:"host_ipv4"`
		System    string `json:"system"`
		Service   string `json:"service"`
		Type      string `json:"type"`
		Resource  string `json:"resource"`
		URL       string `json:"url,omitempty"`
	}

	// RequestMetrics is the metrics of http request.
	RequestMetrics struct {
		GlobalFields

		Count uint64  `json:"cnt"`
		M1    float64 `json:"m1"`
		M5    float64 `json:"m5"`
		M15   float64 `json:"m15"`

		ErrCount uint64  `json:"errcnt"`
		M1Err    float64 `json:"m1err"`
		M5Err    float64 `json:"m5err"`
		M15Err   float64 `json:"m15err"`

		M1ErrPercent  float64 `json:"m1errpct"`
		M5ErrPercent  float64 `json:"m5errpct"`
		M15ErrPercent float64 `json:"m15errpct"`

		Min  uint64 `json:"min"`
		Max  uint64 `json:"max"`
		Mean uint64 `json:"mean"`

		P25  float64 `json:"p25"`
		P50  float64 `json:"p50"`
		P75  float64 `json:"p75"`
		P95  float64 `json:"p95"`
		P98  float64 `json:"p98"`
		P99  float64 `json:"p99"`
		P999 float64 `json:"p999"`

		ReqSize  uint64 `json:"reqsize"`
		RespSize uint64 `json:"respsize"`
	}

	// StatusCodeMetrics is the metrics of http status code.
	StatusCodeMetrics struct {
		GlobalFields

		Code  int    `json:"code"`
		Count uint64 `json:"cnt"`
	}
)

// DefaultSpec returns EaseMonitorMetrics default spec.
func DefaultSpec() *Spec {
	return &Spec{
		Kafka: &KafkaSpec{
			Brokers: []string{"localhost:9092"},
		},
	}
}

// Validate validates Spec.
func (spec Spec) Validate() error {
	return nil
}

// New creates an EaseMonitorMetrics.
func New(spec *Spec, prev *EaseMonitorMetrics, handlers *sync.Map) *EaseMonitorMetrics {
	emm := &EaseMonitorMetrics{
		spec: spec,
		done: make(chan struct{}),
	}
	if prev != nil {
		prev.Close()
	}

	_, err := emm.getClient()
	if err != nil {
		logger.Errorf("%s get kafka producer client failed: %v", spec.Name, err)
	}

	go emm.run()

	return emm
}

func (emm *EaseMonitorMetrics) getClient() (sarama.AsyncProducer, error) {
	client := emm.client.Load()
	if client != nil {
		return client.(sarama.AsyncProducer), nil
	}

	emm.clientMutex.Lock()
	defer emm.clientMutex.Unlock()

	// NOTE: Default config is good enough for now.
	config := sarama.NewConfig()
	config.ClientID = emm.spec.Name
	config.Version = sarama.V0_10_2_0

	producer, err := sarama.NewAsyncProducer(emm.spec.Kafka.Brokers, config)
	if err != nil {
		return nil, fmt.Errorf("start sarama producer failed(brokers: %v): %v",
			emm.spec.Kafka.Brokers, err)
	}

	go func() {
		for {
			select {
			case <-emm.done:
				return
			case err, ok := <-producer.Errors():
				if !ok {
					return
				}
				logger.Errorf("produce failed:", err)
			}
		}
	}()

	emm.client.Store(producer)

	logger.Infof("%s build kafka producer successfully", emm.spec.Name)

	return producer, nil
}

func (emm *EaseMonitorMetrics) closeClient() {
	emm.clientMutex.Lock()
	defer emm.clientMutex.Unlock()

	value := emm.client.Load()
	if value == nil {
		return
	}
	client := value.(sarama.AsyncProducer)

	err := client.Close()
	if err != nil {
		logger.Errorf("%s close kafka producer failed: %v", emm.spec.Name, err)
	}
}

func (emm *EaseMonitorMetrics) run() {
	for {
		select {
		case <-emm.done:
			return
		case <-time.After(scheduler.SyncStatusPaceInUnixSeconds * time.Second):
			client, err := emm.getClient()
			if err != nil {
				logger.Errorf("%s get kafka producer failed: %v",
					emm.spec.Name, err)
				continue
			}

			records := scheduler.Global.GetStatusesRecords()
			for _, record := range records {
				if record.UnixTimestmp <= emm.latestTimestamp {
					continue
				}
				messages := emm.record2Messages(record)

				for _, message := range messages {
					client.Input() <- message
				}

				if err != nil {
					emm.latestTimestamp = record.UnixTimestmp
				}
			}
		}
	}
}

func (emm *EaseMonitorMetrics) record2Messages(record *scheduler.StatusesRecord) []*sarama.ProducerMessage {
	reqMetrics := []*RequestMetrics{}
	codeMetrics := []*StatusCodeMetrics{}

	for objectName, status := range record.Statuses {
		baseFields := &GlobalFields{
			Timestamp: record.UnixTimestmp * 1000,
			Category:  "application",
			HostName:  option.Global.Name,
			HostIpv4:  hostIPv4,
			System:    option.Global.ClusterName,
			Service:   objectName,
		}

		switch status := status.(type) {
		case *httppipeline.Status:
			reqs, codes := emm.httpPipeline2Metrics(baseFields, status)
			reqMetrics = append(reqMetrics, reqs...)
			codeMetrics = append(codeMetrics, codes...)
		case *httpserver.Status:
			reqs, codes := emm.httpServer2Metrics(baseFields, status)
			reqMetrics = append(reqMetrics, reqs...)
			codeMetrics = append(codeMetrics, codes...)
		default:
			continue
		}

	}

	metrics := [][]byte{}
	for _, req := range reqMetrics {
		buff, err := jsoniter.Marshal(req)
		if err != nil {
			logger.Errorf("marshal %#v to json failed: %v", req, err)
		}
		metrics = append(metrics, buff)
	}
	for _, code := range codeMetrics {
		buff, err := jsoniter.Marshal(code)
		if err != nil {
			logger.Errorf("marshal %#v to json failed: %v", code, err)
		}
		metrics = append(metrics, buff)
	}

	messages := make([]*sarama.ProducerMessage, len(metrics))
	for i, metric := range metrics {
		messages[i] = &sarama.ProducerMessage{
			Topic: emm.spec.Kafka.Topic,
			Value: sarama.ByteEncoder(metric),
		}
	}

	return messages
}

func (emm *EaseMonitorMetrics) httpPipeline2Metrics(
	baseFields *GlobalFields, pipelineStatus *httppipeline.Status) (
	reqMetrics []*RequestMetrics, codeMetrics []*StatusCodeMetrics) {

	for pluginName, pluginStatus := range pipelineStatus.Plugins {
		backendStatus, ok := pluginStatus.(*backend.Status)
		if !ok {
			continue
		}

		baseFieldsBackend := *baseFields
		baseFieldsBackend.Resource = "BACKEND"

		if backendStatus.MainPool != nil {
			baseFieldsBackend.Service = baseFields.Service + "/" + pluginName + "/mainPool"
			req, codes := emm.httpStat2Metrics(&baseFieldsBackend, backendStatus.MainPool.Stat)
			reqMetrics = append(reqMetrics, req)
			codeMetrics = append(codeMetrics, codes...)
		}

		if backendStatus.CandidatePool != nil {
			baseFieldsBackend.Service = baseFields.Service + "/" + pluginName + "/candidatePool"
			req, codes := emm.httpStat2Metrics(&baseFieldsBackend, backendStatus.MainPool.Stat)
			reqMetrics = append(reqMetrics, req)
			codeMetrics = append(codeMetrics, codes...)
		}

		if backendStatus.MirrorPool != nil {
			baseFieldsBackend.Service = baseFields.Service + "/" + pluginName + "/mirrorPool"
			req, codes := emm.httpStat2Metrics(&baseFieldsBackend, backendStatus.MainPool.Stat)
			reqMetrics = append(reqMetrics, req)
			codeMetrics = append(codeMetrics, codes...)
		}

	}

	return
}

func (emm *EaseMonitorMetrics) httpServer2Metrics(
	baseFields *GlobalFields, serverStatus *httpserver.Status) (
	reqMetrics []*RequestMetrics, codeMetrics []*StatusCodeMetrics) {

	if serverStatus.Status != nil {
		baseFieldsServer := *baseFields
		baseFieldsServer.Resource = "SERVER"
		req, codes := emm.httpStat2Metrics(&baseFieldsServer, serverStatus.Status)
		reqMetrics = append(reqMetrics, req)
		codeMetrics = append(codeMetrics, codes...)
	}

	for _, item := range *serverStatus.TopN {
		baseFieldsServerTopN := *baseFields
		baseFieldsServerTopN.Resource = "SERVER_TOPN"
		baseFieldsServerTopN.URL = item.Path
		req, codes := emm.httpStat2Metrics(&baseFieldsServerTopN, item.Status)
		reqMetrics = append(reqMetrics, req)
		codeMetrics = append(codeMetrics, codes...)
	}

	return
}

func (emm *EaseMonitorMetrics) httpStat2Metrics(baseFields *GlobalFields, s *httpstat.Status) (
	*RequestMetrics, []*StatusCodeMetrics) {

	baseFields.Type = "eg-http-request"
	rm := &RequestMetrics{
		GlobalFields: *baseFields,

		Count: s.Count,
		M1:    s.M1,
		M5:    s.M5,
		M15:   s.M15,

		ErrCount: s.ErrCount,
		M1Err:    s.M1Err,
		M5Err:    s.M5Err,
		M15Err:   s.M15Err,

		M1ErrPercent:  s.M1ErrPercent,
		M5ErrPercent:  s.M5ErrPercent,
		M15ErrPercent: s.M15ErrPercent,

		Min:  s.Min,
		Max:  s.Max,
		Mean: s.Mean,

		P25:  s.P25,
		P50:  s.P50,
		P75:  s.P75,
		P95:  s.P95,
		P98:  s.P98,
		P99:  s.P99,
		P999: s.P999,

		ReqSize:  s.ReqSize,
		RespSize: s.RespSize,
	}

	baseFields.Type = "eg-http-status-code"
	codes := []*StatusCodeMetrics{}
	for code, count := range s.Codes {
		codes = append(codes, &StatusCodeMetrics{
			GlobalFields: *baseFields,
			Code:         code,
			Count:        count,
		})
	}

	return rm, codes
}

// Status returns status of EtcdServiceRegister.
func (emm *EaseMonitorMetrics) Status() *Status {
	s := &Status{}

	_, err := emm.getClient()
	if err != nil {
		s.Health = err.Error()
	} else {
		s.Health = "ready"
	}

	return s
}

// Close closes EaseMonitorMetrics.
func (emm *EaseMonitorMetrics) Close() {
	// NOTE: close the channel first in case of
	// using closed client in the run().
	close(emm.done)
	emm.closeClient()
}

func getHostIPv4() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		panic(err)
	}

	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			panic(err)
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			ipv4 := ip.To4()
			if !ip.IsLoopback() && ipv4 != nil {
				return ipv4.String()
			}
		}
	}

	return ""
}
