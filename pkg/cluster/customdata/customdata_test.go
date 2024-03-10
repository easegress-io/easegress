/*
 * Copyright (c) 2017, The Easegress Authors
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package customdata

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/megaease/easegress/v2/pkg/cluster"
	"github.com/megaease/easegress/v2/pkg/cluster/clustertest"
	"github.com/megaease/easegress/v2/pkg/util/dynamicobject"

	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/client/v3/concurrency"
)

func TestDataID(t *testing.T) {
	k := Kind{}
	data := Data{}
	data["name"] = "abc"
	data["key"] = "key"

	if id := k.dataID(data); id != "abc" {
		t.Errorf("data ID should be 'abc` instead of %q", id)
	}

	k.IDField = "key"
	if id := k.dataID(data); id != "key" {
		t.Errorf("data ID should be 'key` instead of %q", id)
	}
}

func TestUnmarshal(t *testing.T) {
	data := []byte(":%344")

	_, err := unmarshalKind(data)
	if err == nil {
		t.Errorf("unmarshalKind should fail")
	}

	_, err = unmarshalData(data)
	if err == nil {
		t.Errorf("unmarshal should fail")
	}

	data = []byte(`name: kind1
idField: name
jsonSchema:
  type: object`)

	k, err := unmarshalKind(data)
	if err != nil {
		t.Errorf("unmarshalKind should succeeded")
	}
	if k.Name != "kind1" {
		t.Errorf("kind name should be 'kind1' instead of %q", k.Name)
	}
	if k.IDField != "name" {
		t.Errorf("ID field should be 'name' instead of %q", k.IDField)
	}

	data = []byte(`name: data1
field1: 123
`)
	d, err := unmarshalData(data)
	if err != nil {
		t.Errorf("unmarshalData should succeeded")
	}

	if name := d.GetString("name"); name != "data1" {
		t.Errorf("name should be 'data1' instead of %q", name)
	}

	if v := d.Get("field1"); int(v.(float64)) != 123 {
		t.Errorf("field1 should be 123 instead of %v", v)
	}
}

func TestNewStore(t *testing.T) {
	s := NewStore(nil, "/kind/", "/data/")
	if s.cluster != nil {
		t.Error("cluster should be nil")
	}
	if s.KindPrefix != "/kind/" {
		t.Error("KindPrefix should be '/kind'")
	}
	if s.DataPrefix != "/data/" {
		t.Error("DataPrefix should be '/data'")
	}

	if key := s.kindKey("test"); key != "/kind/test" {
		t.Errorf("kind key should be '/kind/test` instead of %q", key)
	}

	if prefix := s.dataPrefix("test"); prefix != "/data/test/" {
		t.Errorf("data prefix should be '/data/test/` instead of %q", prefix)
	}

	if key := s.dataKey("foo", "bar"); key != "/data/foo/bar" {
		t.Errorf("data key should be '/data/foo/bar` instead of %q", key)
	}
}

func TestGetKind(t *testing.T) {
	cls := clustertest.NewMockedCluster()
	s := NewStore(cls, "/kind/", "/data/")

	cls.MockedGetRaw = func(key string) (*mvccpb.KeyValue, error) {
		return nil, fmt.Errorf("mocked error")
	}
	_, err := s.GetKind("kind1")
	if err == nil {
		t.Errorf("GetKind should fail")
	}

	cls.MockedGetRaw = func(key string) (*mvccpb.KeyValue, error) {
		return nil, nil
	}
	k, err := s.GetKind("kind1")
	if err != nil {
		t.Errorf("GetKind should succeed")
	}
	if k != nil {
		t.Errorf("GetKind should return nil")
	}

	cls.MockedGetRaw = func(key string) (*mvccpb.KeyValue, error) {
		return &mvccpb.KeyValue{
			Value: []byte(`name: kind1`),
		}, nil
	}
	k, err = s.GetKind("kind1")
	if err != nil {
		t.Errorf("GetKind should succeed")
	}
	if k == nil {
		t.Errorf("GetKind should return non-nil value")
	}
}

func TestListKinds(t *testing.T) {
	cls := clustertest.NewMockedCluster()
	s := NewStore(cls, "/kind/", "/data/")

	cls.MockedGetRawPrefix = func(prefix string) (map[string]*mvccpb.KeyValue, error) {
		return nil, fmt.Errorf("mocked error")
	}
	_, err := s.ListKinds()
	if err == nil {
		t.Errorf("ListKinds should fail")
	}

	cls.MockedGetRawPrefix = func(prefix string) (map[string]*mvccpb.KeyValue, error) {
		return map[string]*mvccpb.KeyValue{
			"kind1": {Value: []byte(`name: kind1`)},
			"kind2": {Value: []byte(`@3sj3`)},
		}, nil
	}
	_, err = s.ListKinds()
	if err == nil {
		t.Errorf("ListKinds should fail")
	}

	cls.MockedGetRawPrefix = func(prefix string) (map[string]*mvccpb.KeyValue, error) {
		return map[string]*mvccpb.KeyValue{
			"kind1": {Value: []byte(`name: kind1`)},
			"kind2": {
				Value: []byte(`name: kind2
idField: key`),
			},
		}, nil
	}
	kinds, err := s.ListKinds()
	if err != nil {
		t.Errorf("ListKinds shoud succeed")
	}

	if len(kinds) != 2 {
		t.Errorf("ListKinds should return 2 kinds")
	}
}

func TestPutKind(t *testing.T) {
	cls := clustertest.NewMockedCluster()
	s := NewStore(cls, "/kind/", "/data/")

	k := &Kind{JSONSchema: dynamicobject.DynamicObject{"@#": "2938"}}
	err := s.PutKind(k, true)
	if err == nil {
		t.Errorf("PutKind should fail")
	}

	k.JSONSchema = nil
	cls.MockedGetRaw = func(key string) (*mvccpb.KeyValue, error) {
		return nil, fmt.Errorf("mocked error")
	}
	err = s.PutKind(k, true)
	if err == nil {
		t.Errorf("PutKind should fail")
	}

	cls.MockedGetRaw = func(key string) (*mvccpb.KeyValue, error) {
		return nil, nil
	}
	err = s.PutKind(k, true)
	if err == nil {
		t.Errorf("PutKind should fail")
	}

	cls.MockedGetRaw = func(key string) (*mvccpb.KeyValue, error) {
		return &mvccpb.KeyValue{
			Value: []byte(`name: kind1`),
		}, nil
	}
	err = s.PutKind(k, false)
	if err == nil {
		t.Errorf("PutKind should fail")
	}

	err = s.PutKind(k, true)
	if err != nil {
		t.Errorf("PutKind should succeed")
	}
}

func TestDeleteKind(t *testing.T) {
	cls := clustertest.NewMockedCluster()
	s := NewStore(cls, "/kind/", "/data/")

	cls.MockedGetRaw = func(key string) (*mvccpb.KeyValue, error) {
		return nil, fmt.Errorf("mocked error")
	}
	err := s.DeleteKind("kind1")
	if err == nil {
		t.Errorf("DeleteKind should fail")
	}

	cls.MockedGetRaw = func(key string) (*mvccpb.KeyValue, error) {
		return nil, nil
	}
	err = s.DeleteKind("kind1")
	if err == nil {
		t.Errorf("DeleteKind should fail")
	}

	cls.MockedGetRaw = func(key string) (*mvccpb.KeyValue, error) {
		return &mvccpb.KeyValue{
			Value: []byte(`name: kind1`),
		}, nil
	}
	err = s.DeleteKind("kind1")
	if err != nil {
		t.Errorf("DeleteKind should succeed")
	}
}

func TestGetData(t *testing.T) {
	cls := clustertest.NewMockedCluster()
	s := NewStore(cls, "/kind/", "/data/")

	cls.MockedGetRaw = func(key string) (*mvccpb.KeyValue, error) {
		return nil, fmt.Errorf("mocked error")
	}
	_, err := s.GetData("kind1", "data1")
	if err == nil {
		t.Errorf("GetData should fail")
	}

	cls.MockedGetRaw = func(key string) (*mvccpb.KeyValue, error) {
		return nil, nil
	}
	d, err := s.GetData("kind1", "data1")
	if err != nil {
		t.Errorf("GetData should succeed")
	}
	if d != nil {
		t.Errorf("GetData should return nil")
	}

	cls.MockedGetRaw = func(key string) (*mvccpb.KeyValue, error) {
		return &mvccpb.KeyValue{
			Value: []byte(`name: data1`),
		}, nil
	}
	d, err = s.GetData("kind1", "data1")
	if err != nil {
		t.Errorf("GetData should succeed")
	}
	if d == nil {
		t.Errorf("GetData should return non-nil value")
	}
}

func TestListData(t *testing.T) {
	cls := clustertest.NewMockedCluster()
	s := NewStore(cls, "/kind/", "/data/")

	cls.MockedGetRawPrefix = func(prefix string) (map[string]*mvccpb.KeyValue, error) {
		return nil, fmt.Errorf("mocked error")
	}
	_, err := s.ListData("")
	if err == nil {
		t.Errorf("ListData should fail")
	}
	_, err = s.ListData("kind1")
	if err == nil {
		t.Errorf("ListData should fail")
	}

	cls.MockedGetRawPrefix = func(prefix string) (map[string]*mvccpb.KeyValue, error) {
		return map[string]*mvccpb.KeyValue{
			"data1": {Value: []byte(`name: data1`)},
			"data2": {Value: []byte(`@3sj3`)},
		}, nil
	}
	_, err = s.ListData("kind1")
	if err == nil {
		t.Errorf("ListData should fail")
	}

	cls.MockedGetRawPrefix = func(prefix string) (map[string]*mvccpb.KeyValue, error) {
		return map[string]*mvccpb.KeyValue{
			"data1": {Value: []byte(`name: data1`)},
			"data2": {
				Value: []byte(`name: data2
field1: 123`),
			},
		}, nil
	}
	data, err := s.ListData("kind1")
	if err != nil {
		t.Errorf("ListData shoud succeed")
	}

	if len(data) != 2 {
		t.Errorf("ListData should return 2 data items")
	}
}

func TestPutData(t *testing.T) {
	cls := clustertest.NewMockedCluster()
	s := NewStore(cls, "/kind/", "/data/")

	// failed to get kind
	cls.MockedGetRaw = func(key string) (*mvccpb.KeyValue, error) {
		return nil, fmt.Errorf("mocked error")
	}
	_, err := s.PutData("kind1", nil, true)
	if err == nil {
		t.Errorf("PutData should fail")
	}

	// kind not found
	cls.MockedGetRaw = func(key string) (*mvccpb.KeyValue, error) {
		return nil, nil
	}
	_, err = s.PutData("kind1", nil, true)
	if err == nil {
		t.Errorf("PutData should fail")
	}

	// empty data id
	cls.MockedGetRaw = func(key string) (*mvccpb.KeyValue, error) {
		return &mvccpb.KeyValue{
			Value: []byte(`name: kind1
jsonSchema:
  type: object
  properties:
    field1:
      type: string
      required: true
`),
		}, nil
	}
	d := Data{}
	_, err = s.PutData("kind1", d, true)
	if err == nil {
		t.Errorf("PutData should fail")
	}

	// validation failure
	d["name"] = "data1"
	_, err = s.PutData("kind1", d, true)
	if err == nil {
		t.Errorf("PutData should fail")
	}

	// make validation success
	d["field1"] = "abc"

	// get data fail
	cls.MockedGetRaw = func(key string) (*mvccpb.KeyValue, error) {
		return nil, fmt.Errorf("mocked error")
	}
	_, err = s.PutData("kind1", d, true)
	if err == nil {
		t.Errorf("PutData should fail")
	}

	// get nil data
	cls.MockedGetRaw = func(key string) (*mvccpb.KeyValue, error) {
		if strings.HasSuffix(key, "data1") {
			return nil, nil
		}
		return &mvccpb.KeyValue{Value: []byte(`name: kind1`)}, nil
	}
	_, err = s.PutData("kind1", d, true)
	if err == nil {
		t.Errorf("PutData should fail")
	}

	// get data success
	cls.MockedGetRaw = func(key string) (*mvccpb.KeyValue, error) {
		if strings.HasSuffix(key, "data1") {
			return &mvccpb.KeyValue{Value: []byte(`name: data1`)}, nil
		}
		return &mvccpb.KeyValue{Value: []byte(`name: kind1`)}, nil
	}
	_, err = s.PutData("kind1", d, false)
	if err == nil {
		t.Errorf("PutData should fail")
	}

	// update success
	id, err := s.PutData("kind1", d, true)
	if err != nil {
		t.Errorf("PutData should succeed")
	}
	if id != "data1" {
		t.Errorf("data ID should be 'data1' instead of %q", id)
	}

	// put data failure
	cls.MockedPut = func(key, value string) error {
		return fmt.Errorf("mocked error")
	}
	_, err = s.PutData("kind1", d, true)
	if err == nil {
		t.Errorf("PutData should fail")
	}
}

func TestBatchUpdateData(t *testing.T) {
	cls := clustertest.NewMockedCluster()
	s := NewStore(cls, "/kind/", "/data/")

	// failed to get kind
	cls.MockedGetRaw = func(key string) (*mvccpb.KeyValue, error) {
		return nil, fmt.Errorf("mocked error")
	}
	err := s.BatchUpdateData("kind1", nil, nil)
	if err == nil {
		t.Errorf("BatchUpdateData should fail")
	}

	// kind not found
	cls.MockedGetRaw = func(key string) (*mvccpb.KeyValue, error) {
		return nil, nil
	}
	err = s.BatchUpdateData("kind1", nil, nil)
	if err == nil {
		t.Errorf("BatchUpdateData should fail")
	}

	data := []Data{
		{"name": "data1", "field1": "foo"},
		{"field2": "field2"},
	}
	// validation failure
	cls.MockedGetRaw = func(key string) (*mvccpb.KeyValue, error) {
		return &mvccpb.KeyValue{
			Value: []byte(`name: kind1
jsonSchema:
  type: object
  properties:
    field1:
      type: string
      required: true
`),
		}, nil
	}
	err = s.BatchUpdateData("kind1", nil, data)
	if err == nil {
		t.Errorf("BatchUpdateData should fail")
	}

	// empty data id
	cls.MockedGetRaw = func(key string) (*mvccpb.KeyValue, error) {
		return &mvccpb.KeyValue{
			Value: []byte(`name: kind1`),
		}, nil
	}
	err = s.BatchUpdateData("kind1", nil, data)
	if err == nil {
		t.Errorf("BatchUpdateData should fail")
	}

	// make validation success
	cls.MockedSTM = func(apply func(concurrency.STM) error) error {
		stm := &clustertest.MockedSTM{}
		return apply(stm)
	}
	data[1]["name"] = "data2"
	err = s.BatchUpdateData("kind1", []string{"data1", "data3"}, data)
	if err != nil {
		t.Errorf("BatchUpdateData should succeed")
	}
}

func TestDeleteData(t *testing.T) {
	cls := clustertest.NewMockedCluster()
	s := NewStore(cls, "/kind/", "/data/")

	cls.MockedGetRaw = func(key string) (*mvccpb.KeyValue, error) {
		return nil, fmt.Errorf("mocked error")
	}
	err := s.DeleteData("kind1", "data1")
	if err == nil {
		t.Errorf("DeleteData should fail")
	}

	cls.MockedGetRaw = func(key string) (*mvccpb.KeyValue, error) {
		if strings.HasSuffix(key, "data1") {
			return nil, nil
		}
		return &mvccpb.KeyValue{Value: []byte(`name: kind1`)}, nil
	}
	err = s.DeleteData("kind1", "data1")
	if err == nil {
		t.Errorf("DeleteData should fail")
	}

	cls.MockedGetRaw = func(key string) (*mvccpb.KeyValue, error) {
		if strings.HasSuffix(key, "data1") {
			return &mvccpb.KeyValue{Value: []byte(`name: data1`)}, nil
		}
		return &mvccpb.KeyValue{Value: []byte(`name: kind1`)}, nil
	}
	err = s.DeleteData("kind1", "data1")
	if err != nil {
		t.Errorf("DeleteData should succeed")
	}
}

func TestDeleteAllData(_ *testing.T) {
	cls := clustertest.NewMockedCluster()
	s := NewStore(cls, "/kind/", "/data/")
	s.DeleteAllData("kind1")
}

func TestWatch(t *testing.T) {
	cls := clustertest.NewMockedCluster()
	s := NewStore(cls, "/kind/", "/data/")

	cls.MockedSyncer = func(pullInterval time.Duration) (cluster.Syncer, error) {
		return nil, fmt.Errorf("mocked error")
	}

	ctx, cancel := context.WithCancel(context.Background())
	err := s.Watch(ctx, "kind1", func(d []Data) {})
	if err == nil {
		t.Errorf("Watch should fail")
	}

	cancel()
}
