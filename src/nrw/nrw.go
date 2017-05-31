package nrw

import (
	"fmt"

	"cluster"
	"common"
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

	chanSize = 100
)

type Config struct {
	// MaxSeqGapToSync means ... TODO
	MaxSeqGapToSync uint64
}

type NRW struct {
	conf    *Config
	model   *model.Model
	cluster *cluster.Cluster
	opLog   *opLog
	mode    Mode

	eventStream            chan cluster.Event
	writeModeOperationChan chan *cluster.RequestEvent
	readModeOperationChan  chan *cluster.RequestEvent
	syncOPLogChan          chan *cluster.RequestEvent
}

func NewNRW(conf *Config, mdl *model.Model) (*NRW, error) {
	if mdl == nil {
		return nil, fmt.Errorf("model is nil")
	}
	// TODO: choose config automatically
	cltConf := cluster.DefaultLocalConfig()
	eventStream := make(chan cluster.Event, chanSize)
	cltConf.EventStream = eventStream
	cltConf.NodeTags["group"] = "default"            // TODO: read from config
	cltConf.NodeTags["nrw_mode"] = string(WriteMode) // TODO: read from config

	clt, err := cluster.Create(cltConf)
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

		eventStream:            eventStream,
		writeModeOperationChan: make(chan *cluster.RequestEvent, chanSize),
		readModeOperationChan:  make(chan *cluster.RequestEvent, chanSize),
		syncOPLogChan:          make(chan *cluster.RequestEvent, chanSize),
	}

	// The 3 must handle corresponding requests one by one sequentially.
	go nrw.handleWriteModeOperation()
	go nrw.handleReadModeOperation()
	go nrw.syncOPLog()

	go nrw.dispatch()

	return nrw, nil
}

// nrw.mode can be changed after re-electing new only one write-mode node.
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
			case MessageRetrieve:
				go nrw.handleRetrieve(event)
			case MessasgeStat:
				go nrw.handleStat(event)

			case MessageOperation: // only execute sequentially
				go func() {
					if nrw.Mode() == WriteMode {
						nrw.writeModeOperationChan <- event
					} else if nrw.Mode() == ReadMode {
						nrw.readModeOperationChan <- event
					}
				}()
			case MessageSyncOPLog:
				go func() {
					nrw.syncOPLogChan <- event
				}()
			}
		case *cluster.MemberEvent:
			// TODO: notify current executing updateBusiness or let it go
			// Maybe use observer pattern.
		}
	}
}

func (nrw *NRW) handleQueryMaxSeq(req *cluster.RequestEvent) {
	// TODO
}

func (nrw *NRW) handleRetrieve(req *cluster.RequestEvent) {
	// TODO
}

func (nrw *NRW) handleStat(req *cluster.RequestEvent) {
	// TODO
}

func (nrw *NRW) handleWriteModeOperation() {
	for {
		req := <-nrw.writeModeOperationChan
		// TODO: when the request has been gossiped, make a new goroutine
		// to wait reponse then respond to rest server.
		_ = req
	}

}

func (nrw *NRW) handleReadModeOperation() {
	for {
		req := <-nrw.readModeOperationChan
		// TODO
		_ = req
	}
}

func (nrw *NRW) syncOPLog() {
	for {
		req := <-nrw.syncOPLogChan
		// TODO
		_ = req
	}
}

func (nrw *NRW) Close() {
	nrw.opLog.close()
}
