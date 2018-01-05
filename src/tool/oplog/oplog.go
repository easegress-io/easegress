package oplog

import (
	"encoding/json"
	"fmt"

	"github.com/urfave/cli"

	"cluster/gateway"
)

type opLogs struct {
	MaxSeq uint64 `json:"current_max_sequence"`
	Path    string `json:"path"`
	Begin  uint64 `json:"begin"`
	Count  uint64 `json:"count"`

	Operations []json.RawMessage `json:"operations"`
}

func RetrieveOpLog(c *cli.Context) error {
	opLogPath := c.String("path")
	opLog, err := gateway.NewOPLog(opLogPath)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("open oplog failed: %v", err), -1)
	}
	defer opLog.Close()

	maxSeq := opLog.MaxSeq()
	startSeq := c.Uint64("begin")
	countLimit := c.Uint64("count")
	if countLimit > maxSeq {
		countLimit = maxSeq
	}

	oplogs := new(opLogs)
	oplogs.MaxSeq = maxSeq
	oplogs.Path = opLog.Path()
	oplogs.Begin = startSeq
	oplogs.Count = countLimit

	operations, err, _ := opLog.Retrieve(startSeq, countLimit)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("retrieve failed: %v", err), -1)
	}

	oplogs.Operations = make([]json.RawMessage, len(operations))
	var byt []byte
	for i, op := range operations {
		if byt, err = op.ToHumanReadableJSON(); err != nil {
			return cli.NewExitError(fmt.Sprintf("marshal operation %v failed: %v", op, err), -3)
		}
		oplogs.Operations[i] = byt
	}

	if byt, err = json.Marshal(oplogs); err != nil {
		return cli.NewExitError(fmt.Sprintf("marshal oplogs failed: %v", err), -3)
	}
	fmt.Printf("%s", byt)
	return nil
}
