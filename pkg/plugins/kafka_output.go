package plugins

import (
	"fmt"
	"strings"
	"time"

	"github.com/hexdecteam/easegateway/pkg/common"
	"github.com/hexdecteam/easegateway/pkg/logger"

	"github.com/Shopify/sarama"
	"github.com/hexdecteam/easegateway-types/pipelines"
	"github.com/hexdecteam/easegateway-types/plugins"
	"github.com/hexdecteam/easegateway-types/task"
)

type kafkaOutputConfig struct {
	common.PluginCommonConfig
	Brokers  []string `json:"brokers"`
	ClientID string   `json:"client_id"`
	Topic    string   `json:"topic"`

	MessageKeyKey string `json:"message_key_key"`
	DataKey       string `json:"data_key"`
}

func kafkaOutputConfigConstructor() plugins.Config {
	return &kafkaOutputConfig{
		ClientID: "easegateway",
	}
}

func (c *kafkaOutputConfig) Prepare(pipelineNames []string) error {
	err := c.PluginCommonConfig.Prepare(pipelineNames)
	if err != nil {
		return err
	}

	ts := strings.TrimSpace
	c.ClientID = ts(c.ClientID)
	c.Topic = ts(c.Topic)
	c.MessageKeyKey = ts(c.MessageKeyKey)
	c.DataKey = ts(c.DataKey)

	if len(c.ClientID) == 0 {
		return fmt.Errorf("invalid kafka client id")
	}

	if len(c.Topic) == 0 {
		return fmt.Errorf("invalid kafka topic")
	}

	if len(c.Brokers) == 0 {
		return fmt.Errorf("invalid kafka borker list")
	}

	for _, broker := range c.Brokers {
		if len(ts(broker)) == 0 {
			return fmt.Errorf("invalid kafka borker name")
		}
	}
	if len(c.DataKey) == 0 {
		return fmt.Errorf("invalid data key")
	}

	return nil
}

type kafkaOutput struct {
	conf     *kafkaOutputConfig
	client   sarama.Client
	producer sarama.SyncProducer
}

func kafkaOutputConstructor(conf plugins.Config) (plugins.Plugin, plugins.PluginType, bool, error) {
	c, ok := conf.(*kafkaOutputConfig)
	if !ok {
		return nil, plugins.SinkPlugin, false, fmt.Errorf(
			"config type want *kafkaOutputConfig got %T", conf)
	}

	k := &kafkaOutput{
		conf: c,
	}

	err := k.connectBroker()
	if err != nil {
		logger.Errorf("[%v connect kafka brokers %v failed: %s]", k.Name(), k.conf.Brokers, err)
		return nil, plugins.SinkPlugin, false, fmt.Errorf("kafka out of service")
	}

	logger.Infof("[%v connect kafka broker(s) %v succeed]", k.Name(), k.conf.Brokers)

	k.producer, err = sarama.NewSyncProducerFromClient(k.client)
	if err != nil {
		logger.Errorf("[%v new producer from %v failed: %s]", k.Name(), k.conf.Brokers, err)
		return nil, plugins.SinkPlugin, false, fmt.Errorf("send message to kafka failed")
	}

	return k, plugins.SinkPlugin, false, nil
}

func (o *kafkaOutput) Prepare(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (o *kafkaOutput) connectBroker() error {
	saramaConfig := sarama.NewConfig()
	saramaConfig.Producer.Return.Successes = true
	saramaConfig.Producer.Partitioner = sarama.NewHashPartitioner
	saramaConfig.ClientID = o.conf.ClientID
	// FIXME: Makes retry stuff to be configurable when needed
	saramaConfig.Metadata.Retry.Max = 1
	saramaConfig.Metadata.Retry.Backoff = 100 * time.Millisecond

	var err error
	err = saramaConfig.Validate()
	if err != nil {
		return err
	}

	o.client, err = sarama.NewClient(o.conf.Brokers, saramaConfig)
	if err != nil {
		return fmt.Errorf(err.Error())
	}

	partitions, err := o.client.Partitions(o.conf.Topic)
	if err != nil {
		return fmt.Errorf(err.Error())
	}

	for _, partition := range partitions {
		_, err = o.client.GetOffset(o.conf.Topic, partition, sarama.OffsetNewest)
		if err != nil {
			return fmt.Errorf(err.Error())
		}
	}

	return nil
}

func (o *kafkaOutput) output(t task.Task) (error, task.TaskResultCode, task.Task) {
	// defensive programming
	if o.producer == nil {
		return fmt.Errorf("kafka producer not ready"), task.ResultServiceUnavailable, t
	}

	dataValue := t.Value(o.conf.DataKey)
	data, ok := dataValue.([]byte)
	if !ok {
		return fmt.Errorf("input %s got wrong value: %#v", o.conf.DataKey, dataValue),
			task.ResultMissingInput, t
	}

	msg := &sarama.ProducerMessage{
		Topic: o.conf.Topic,
		Value: sarama.ByteEncoder(data),
	}

	if len(o.conf.MessageKeyKey) != 0 {
		messageKeyValue := t.Value(o.conf.MessageKeyKey)
		messageKey, ok := messageKeyValue.(string)
		if !ok {
			return fmt.Errorf("input %s got wrong value: %#v", o.conf.MessageKeyKey, messageKeyValue),
				task.ResultMissingInput, t
		}
		messageKey = strings.TrimSpace(messageKey)
		if len(messageKey) == 0 {
			return fmt.Errorf("input %s got empty string", o.conf.MessageKeyKey),
				task.ResultBadInput, t
		}
		msg.Key = sarama.StringEncoder(messageKey)
	}

	_, _, err := o.producer.SendMessage(msg)
	if err != nil {
		logger.Warnf("[send message to topic (%s) failed]", o.conf.Topic)
		return err, task.ResultServiceUnavailable, t
	}

	return nil, t.ResultCode(), t
}

// lists errors gateway can swallow, and needn't to reconstruct plugin
var kafkaOutputErrorsTolerable = []string{
	sarama.ErrMessageSizeTooLarge.Error(),
}

func (o *kafkaOutput) Run(ctx pipelines.PipelineContext, t task.Task) error {
	err, resultCode, t := o.output(t)
	if err != nil {
		if resultCode == task.ResultServiceUnavailable &&
			!common.StrInSlice(err.Error(), kafkaOutputErrorsTolerable) {
			return err
		} else {
			t.SetError(err, resultCode)
			return nil
		}
	}
	return nil
}

func (o *kafkaOutput) Name() string {
	return o.conf.PluginName()
}

func (o *kafkaOutput) CleanUp(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (o *kafkaOutput) Close() {
	err := o.producer.Close()
	if err != nil {
		logger.Errorf("[close produce failed: %v]", err)
	}

	err = nil
	err = o.client.Close()
	if err != nil {
		logger.Errorf("[close client failed: %v]", err)
	}
}
