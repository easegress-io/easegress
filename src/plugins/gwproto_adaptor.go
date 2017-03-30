package plugins

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"task"

	"github.com/golang/protobuf/proto"

	"pipelines"
	"plugins/gwproto"
)

type gwProtoAdaptorConfig struct {
	CommonConfig

	GidKey  string `json:"gid_key"`
	DataKey string `json:"data_key"`
}

func GWProtoAdaptorConfigConstructor() Config {
	return &gwProtoAdaptorConfig{}
}

func (c *gwProtoAdaptorConfig) Prepare() error {
	err := c.CommonConfig.Prepare()
	if err != nil {
		return err
	}

	ts := strings.TrimSpace
	c.GidKey, c.DataKey = ts(c.GidKey), ts(c.DataKey)

	if len(c.GidKey) == 0 {
		return fmt.Errorf("invalid gid key")
	}

	if len(c.DataKey) == 0 {
		return fmt.Errorf("invalid data key")
	}

	return nil
}

type gwProtoAdaptor struct {
	conf *gwProtoAdaptorConfig
}

func GWProtoAdaptorConstructor(conf Config) (Plugin, error) {
	c, ok := conf.(*gwProtoAdaptorConfig)
	if !ok {
		return nil, fmt.Errorf("config type want *gwProtoAdaptorConfig got %T", conf)
	}

	return &gwProtoAdaptor{
		conf: c,
	}, nil
}

func (a *gwProtoAdaptor) Prepare(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (a *gwProtoAdaptor) adapt(t task.Task) (err error, resultCode task.TaskResultCode, retTask task.Task) {
	dataValue := t.Value(a.conf.DataKey)
	data, ok := dataValue.([]byte)
	if !ok {
		return fmt.Errorf("input %s got wrong value: %#v", a.conf.DataKey, dataValue), task.ResultMissingInput, t
	}

	const (
		MAGIC_NUMBER byte = 0x4D

		MAGIC_SIZE    uint64 = 1
		PBLENGTH_SIZE uint64 = 4
	)

	var (
		MAGIC_PARSER    = func(b []byte) byte { return b[0] }
		PBLENGTH_PARSER = func(b []byte) uint64 { return uint64(binary.BigEndian.Uint32(b)) }

		i       uint64
		count   uint64
		n       uint64 = uint64(len(data))
		newData []byte
		gid     string
	)

	resultCode = task.ResultBadInput

	if n == 0 {
		err = fmt.Errorf("unexpected EOF")
		return
	}

	for {
		if i >= n {
			break
		}
		count++

		// magic number
		if i+MAGIC_SIZE > n {
			err = fmt.Errorf("index %v magic unexpected EOF: %v > %v\n", count, i+MAGIC_SIZE, n)
			return
		}
		magic := MAGIC_PARSER(data[i : i+MAGIC_SIZE])
		if magic != MAGIC_NUMBER {
			err = fmt.Errorf("index %v data[%v:%v] %v is unequal to magic number %v",
				count, i, i+MAGIC_SIZE, magic, MAGIC_NUMBER)
			return
		}
		i += MAGIC_SIZE

		// protobuf length
		if i+PBLENGTH_SIZE > n {
			err = fmt.Errorf("index %v protobuf length unexpected EOF: %v > %v\n",
				count, i+PBLENGTH_SIZE, n)
			return
		}
		pbLength := PBLENGTH_PARSER(data[i : i+PBLENGTH_SIZE])
		i += PBLENGTH_SIZE

		// protobuf data
		if i+pbLength > n {
			err = fmt.Errorf("index %v protobuf data unexpexted EOF: %v > %v\n", count, i+pbLength, n)
			return
		}
		pbData := data[i : i+pbLength]
		apmData := new(gwproto.ApmData)
		e := proto.Unmarshal(pbData, apmData)
		if e != nil {
			err = fmt.Errorf("index %v unmarshal protobuf data failed: %v", count, err)
			return
		}
		i += pbLength

		gid = apmData.Infra.Device

		apmDataJson, e := json.Marshal(apmData)
		if e != nil {
			err = fmt.Errorf("index %v marshal protobuf data to json failed: %v", count, err)
			return
		}

		newData = append(newData, apmDataJson...)
		newData = append(newData, '\n')
	}

	resultCode = t.ResultCode()

	retTask = t
	retTask, err = task.WithValue(retTask, a.conf.GidKey, gid)
	if err != nil {
		resultCode = task.ResultInternalServerError
		return
	}

	retTask, err = task.WithValue(retTask, a.conf.DataKey, newData)
	if err != nil {
		resultCode = task.ResultInternalServerError
		return
	}

	return
}

func (a *gwProtoAdaptor) Run(ctx pipelines.PipelineContext, t task.Task) (task.Task, error) {
	err, resultCode, t := a.adapt(t)
	if err != nil {
		t.SetError(err, resultCode)
	}
	return t, nil
}

func (g *gwProtoAdaptor) Name() string {
	return g.conf.PluginName()
}

func (g *gwProtoAdaptor) Close() {
	// Nothing to do.
}
