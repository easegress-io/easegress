package oplog

import (
	"encoding/json"
	"fmt"

	"github.com/hexdecteam/easegateway/pkg/cluster/gateway"
	"github.com/hexdecteam/easegateway/pkg/common"

	"github.com/urfave/cli"
)

type opLogs struct {
	TimeStamp string `json:"timestamp"`
	MaxSeq    uint64 `json:"current_max_sequence"`
	Path      string `json:"path"`
	Begin     uint64 `json:"begin"`
	Count     uint64 `json:"count"`

	Operations []json.RawMessage `json:"operations"`
}

func RetrieveOpLog(c *cli.Context) error {
	opLogPath := c.String("path")
	opLog, err := gateway.NewOPLog(opLogPath)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("open oplog failed: %v", err), 1)
	}
	defer opLog.Close()

	maxSeq := opLog.MaxSeq()
	begin := c.Uint64("begin")
	if begin > maxSeq {
		return cli.NewExitError(
			fmt.Sprintf("`begin` parameter invalid, it must be equal or lower than current max sequence: %d", maxSeq),
			3)
	}
	count := c.Uint64("count")
	if maxSeq == 0 {
		count = 0
	} else if begin+count > maxSeq {
		count = maxSeq - begin + 1
	}

	oplogs := opLogs{
		TimeStamp: fmt.Sprintf("%s", common.Now().Local()),
		MaxSeq:    maxSeq,
		Path:      opLog.Path(),
		Begin:     begin,
		Count:     count,
	}

	var operations []*gateway.Operation
	if maxSeq > 0 {
		operations, err, _ = opLog.Retrieve(begin, count)
		if err != nil {
			return cli.NewExitError(fmt.Sprintf("retrieve oplog failed: %v", err), 4)
		}
	}

	oplogs.Operations = make([]json.RawMessage, len(operations))
	var byt []byte
	for i, op := range operations {
		if byt, err = op.ToHumanReadableJSON(); err != nil {
			byt = []byte(fmt.Sprintf("marshal operation failed: %v", err))
		}
		oplogs.Operations[i] = byt
	}

	if byt, err = json.Marshal(oplogs); err != nil {
		return cli.NewExitError(fmt.Sprintf("marshal oplogs failed: %v", err), 5)
	}

	fmt.Printf("%s\n", byt)
	return nil
}
