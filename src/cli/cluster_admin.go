package cli

import (
	"common"
	"fmt"
	"os"

	"github.com/hexdecteam/easegateway-go-client/rest/1.0/cluster/admin/v1/pdu"
	"github.com/urfave/cli"
)

type runtimeConfig struct {
	GroupSeq map[string]uint64 `json:"group_sequence"`
}

func setLocalOperationSequence(group string, seq uint64) error {
	if _, err := os.Stat(rcFullPath); os.IsNotExist(err) {
		file, err := os.OpenFile(rcFullPath, os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			return err
		}
		file.WriteString("{}")
		file.Close()
	}

	store := newRCJSONFileStore(rcFullPath)
	rc, err := store.load()
	if err != nil {
		return err
	}

	if rc.GroupSeq == nil {
		rc.GroupSeq = make(map[string]uint64)
	}
	rc.GroupSeq[group] = seq
	err = store.save(rc)
	if err != nil {
		return err
	}

	return nil
}

func getLocalOperationSequence(group string) (uint64, error) {
	if _, err := os.Stat(rcFullPath); os.IsNotExist(err) {
		return 0, fmt.Errorf("the runtime config file %s doesn't exist", rcFullPath)
	}

	store := newRCJSONFileStore(rcFullPath)
	rc, err := store.load()
	if err != nil {
		return 0, err
	}

	seq, ok := rc.GroupSeq[group]
	if !ok {
		return 0, fmt.Errorf("not found sequence of %s", group)
	}

	return seq, nil
}

func clusterRetrieveOperationSequnce(group string, timeoutSec uint16) (uint64, error) {
	req := new(pdu.ClusterOperationSeqRetrieveRequest)
	req.TimeoutSec = timeoutSec
	retrieveResp, apiResp, err := clusterAdminApi().GetMaxOperationSequence(group, req)
	if err != nil {
		return 0, err
	} else if apiResp.Error != nil {
		return 0, fmt.Errorf("%s", apiResp.Error)
	}
	return retrieveResp.OperationSeq, nil
}

func getOperationSequence(group string, timeoutSec uint16) (uint64, error) {
	localSeq, localErr := getLocalOperationSequence(group)
	if localErr != nil {
		fmt.Printf("Warning: %v\n", localErr)
	}

	remoteSeq, remoteErr := clusterRetrieveOperationSequnce(group, timeoutSec)
	if remoteErr != nil {
		return 0, remoteErr
	} else if localErr != nil {
		return remoteSeq, nil
	}

	switch {
	case localSeq == remoteSeq:
		return localSeq, nil
	case localSeq < remoteSeq:
		fmt.Printf("Warning: the configuration of %s has changed\n", group)
		return remoteSeq, nil
	case localSeq > remoteSeq:
		fmt.Printf("Warning: BUG happended: local sequence %d is "+
			"unexpectedly greater than remote sequence %d", localSeq, remoteSeq)
		return remoteSeq, nil
	}

	return remoteSeq, nil
}

func ClusterCreatePlugin(c *cli.Context) error {
	args := c.Args()
	timeoutSec := uint16(*c.Generic("timeout").(*common.Uint16Value))
	_, _ = args, timeoutSec

	// FIXME: When handling one or more plugins, the timeoutSec should apply
	// to each one or all of them.

	return nil
}

func ClusterDeletePlugin(c *cli.Context) error {
	return nil
}

func ClusterRetrievePlugins(c *cli.Context) error {
	return nil
}

func ClusterUpdatePlugin(c *cli.Context) error {
	return nil
}

func ClusterCreatePipeline(c *cli.Context) error {
	return nil
}

func ClusterDeletePipeline(c *cli.Context) error {
	return nil
}

func ClusterRetrievePipelines(c *cli.Context) error {
	return nil
}

func ClusterUpdatePipeline(c *cli.Context) error {
	return nil
}

func ClusterRetrievePluginTypes(c *cli.Context) error {
	return nil
}

func ClusterRetrievePipelineTypes(c *cli.Context) error {
	return nil
}
