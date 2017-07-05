package cli

import (
	"common"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"time"

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
		return 0, nil
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

	if localSeq < remoteSeq {
		return 0, fmt.Errorf("the remote configure of %s has changed\n", group)
	}

	return remoteSeq, nil
}

func ClusterCreatePlugin(c *cli.Context) error {
	args := c.Args()
	group := c.GlobalString("group")
	timeoutSec := uint16(*c.GlobalGeneric("timeout").(*common.Uint16Value))
	timeout := time.Duration(timeoutSec) * time.Second
	consistent := c.GlobalBool("consistent")

	errs := &multipleErr{}

	do := func(source string, seq uint64, data []byte) {
		req := new(pdu.PluginCreationClusterRequest)
		req.TimeoutSec = uint16(timeout.Seconds())
		req.Consistent = consistent
		req.OperationSeq = seq

		// FIXME: Need easegateway-go-client to wrap req.Type&req.Config
		// into req.PluginCreationRequest
		err := json.Unmarshal(data, req) // for compilation: req.PluginCreationRequest)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", source, err))
			return
		}

		resp, err := clusterAdminApi().CreatePlugin(group, req)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", source, err))
			return
		} else if resp.Error != nil {
			errs.append(fmt.Errorf("%s: %s", source, resp.Error.Error))
			return
		}
	}

	if len(args) == 0 {
		args = append(args, "/dev/stdin")
	}

	for i, file := range args {
		data, err := ioutil.ReadFile(file)
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", file, err))
			continue
		}

		startTime := time.Now()
		seq, err := getOperationSequence(group, uint16(timeout.Seconds()))
		if err != nil {
			errs.append(fmt.Errorf("%s: %v", file, err))
			break
		}
		expiredTime := time.Now().Sub(startTime)
		if timeout <= expiredTime {
			errs.append(fmt.Errorf("timeout: no time to handle", args[i:]))
			break
		}
		timeout -= expiredTime

		startTime = time.Now()
		do(file, seq, data)
		expiredTime = time.Now().Sub(startTime)
		if timeout <= expiredTime && i < len(args)-1 {
			errs.append(fmt.Errorf("timeout: no time to handle: %s", args[i+1:]))
			break
		}
		timeout -= expiredTime
	}

	return errs.Return()
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
