package nrw

import (
	"fmt"
	"time"

	"cluster"
	"common"
	"logger"
	"model"
)

var (
	Pack   = common.Pack
	Unpack = common.Unpack
)

type Mode string

const (
	WriteMode Mode = "WriteMode"
	ReadMode  Mode = "ReadMode"
)

type Config struct {
	// MaxSeqGapToSync means ... TODO
	MaxSeqGapToSync   uint64
	SyncOPLogInterval time.Duration
}

type NRW struct {
	conf    *Config
	model   *model.Model
	cluster *cluster.Cluster
	opLog   *opLog
	mode    Mode

	eventStream chan cluster.Event
}

func NewNRW(conf *Config, mdl *model.Model) (*NRW, error) {
	if mdl == nil {
		return nil, fmt.Errorf("model is nil")
	}
	// TODO: choose config automatically
	cltConf := cluster.DefaultLANConfig()
	eventStream := make(chan cluster.Event)
	cltConf.EventStream = eventStream
	cltConf.NodeTags["group"] = "default"            // TODO: read from config
	cltConf.NodeTags["nrw_mode"] = string(WriteMode) // TODO: read from config

	clt, err := cluster.Create(*cltConf)
	if err != nil {
		return nil, err
	}

	opLog, err := newOPLog()
	if err != nil {
		return nil, err
	}

	nrw := &NRW{
		conf:    conf,
		model:   mdl,
		cluster: clt,
		opLog:   opLog,
		mode:    WriteMode, // TODO

		eventStream: eventStream,
	}

	go nrw.syncOPLog()
	go nrw.dispatch()

	return nrw, nil
}

func (nrw *NRW) Mode() Mode {
	return nrw.mode
}

func (nrw *NRW) dispatch() {
	for {
		event := <-nrw.eventStream
		switch event := event.(type) {
		case *cluster.RequestEvent:
			if len(event.RequestPayload) < 1 {
				break
			}
			msgType := event.RequestPayload[0]

			switch MessageType(msgType) {
			case MessageQueryMaxSeq:
				go nrw.handleQueryMaxSeq(event)
			case MessageOperation:
				if nrw.Mode() == WriteMode {
					go nrw.handleWriteModeOperation(event)
				} else if nrw.Mode() == ReadMode {
					go nrw.handleReadModeOperation(event)
				}
			case MessageRetrieve:
				if nrw.Mode() == WriteMode {
					go nrw.handleWriteModeRetrieve(event)
				} else if nrw.Mode() == ReadMode {
					go nrw.handleReadModeRetrieve(event)
				}
			case MessasgeStat:
				go nrw.handleStat(event)
			case MessageSyncOPLog:
				nrw.handleSyncOPLog(event)
			}
		case *cluster.MemberEvent:
			// TODO: notify current executing updateBusiness or let it go
			// Maybe use observer pattern.
		}
	}
}

func (nrw *NRW) handleQueryMaxSeq(req *cluster.RequestEvent) {
	ms := nrw.opLog.maxSeq()
	payload, err := common.Pack(RespQueryMaxSeq(ms), uint8(MessageQueryMaxSeq))
	if err != nil {
		logger.Errorf("[BUG: pack max sequence %d failed: %v]", ms, err)
		return
	}
	err = req.Respond(payload)
	if err != nil {
		logger.Errorf("[repond reqeust max sequence to request %s, node %s failed: %v]", req.RequestName, req.RequestNodeName, err)
	}
}

func (nrw *NRW) Close() {
	nrw.opLog.close()
}
