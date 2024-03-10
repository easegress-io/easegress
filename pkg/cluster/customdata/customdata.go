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

// Package customdata provides a way to store custom data in Easegress cluster.
package customdata

import (
	"context"
	"fmt"
	"time"

	"github.com/megaease/easegress/v2/pkg/cluster"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/megaease/easegress/v2/pkg/util/dynamicobject"
	"github.com/xeipuuv/gojsonschema"
	"go.etcd.io/etcd/client/v3/concurrency"
)

// Data represents a custom data
type Data = dynamicobject.DynamicObject

// Kind defines the spec of a custom data kind
type Kind struct {
	// Name is the name of the Kind
	Name string `json:"name" jsonschema:"required"`
	// IDField is a field name of custom data of this kind, this field is the ID
	// of the data, that's unique among the same kind, the default value is 'name'.
	IDField string `json:"idField,omitempty"`
	// JSONSchema is JSON schema to validate a custom data of this kind
	JSONSchema dynamicobject.DynamicObject `json:"jsonSchema,omitempty"`
}

// KindWithLen defines the spec of a custom data kind with length
type KindWithLen struct {
	Kind
	Len int `json:"len"`
}

// DataID returns the ID of the custom data
func (k *Kind) DataID(data *Data) string {
	return k.dataID(*data)
}

// GetIDField returns the ID field name of the custom data kind
func (k *Kind) GetIDField() string {
	if k.IDField == "" {
		return "name"
	}
	return k.IDField
}

func (k *Kind) dataID(data Data) string {
	var id string
	if k.IDField == "" {
		id, _ = data["name"].(string)
	} else {
		id, _ = data[k.IDField].(string)
	}
	return id
}

// Store defines the storage for custom data
type Store struct {
	cluster    cluster.Cluster
	KindPrefix string
	DataPrefix string
}

// NewStore creates a new custom data store
func NewStore(cls cluster.Cluster, kindPrefix string, dataPrefix string) *Store {
	return &Store{
		cluster:    cls,
		KindPrefix: kindPrefix,
		DataPrefix: dataPrefix,
	}
}

func unmarshalKind(yamlConfig []byte) (*Kind, error) {
	kind := &Kind{}
	err := codectool.Unmarshal(yamlConfig, kind)
	if err != nil {
		return nil, fmt.Errorf("BUG: unmarshal %s to json failed: %v", string(yamlConfig), err)
	}
	return kind, nil
}

func (s *Store) kindKey(name string) string {
	return s.KindPrefix + name
}

// GetKind gets custom data kind by its name
func (s *Store) GetKind(name string) (*Kind, error) {
	kvs, err := s.cluster.GetRaw(s.kindKey(name))
	if err != nil {
		return nil, err
	}

	if kvs == nil {
		return nil, nil
	}

	return unmarshalKind(kvs.Value)
}

// ListKinds lists custom data kinds
func (s *Store) ListKinds() ([]*Kind, error) {
	kvs, err := s.cluster.GetRawPrefix(s.KindPrefix)
	if err != nil {
		return nil, err
	}

	kinds := make([]*Kind, 0, len(kvs))
	for _, v := range kvs {
		kind, err := unmarshalKind(v.Value)
		if err != nil {
			return nil, err
		}
		kinds = append(kinds, kind)
	}

	return kinds, nil
}

// PutKind creates or updates a custom data kind
func (s *Store) PutKind(kind *Kind, update bool) error {
	if len(kind.JSONSchema) > 0 {
		sl := gojsonschema.NewGoLoader(kind.JSONSchema)
		if _, err := gojsonschema.NewSchema(sl); err != nil {
			return fmt.Errorf("invalid JSONSchema: %s", err.Error())
		}
	}

	oldKind, err := s.GetKind(kind.Name)
	if err != nil {
		return err
	}

	if update && (oldKind == nil) {
		return fmt.Errorf("%s not found", kind.Name)
	}
	if (!update) && (oldKind != nil) {
		return fmt.Errorf("%s existed", kind.Name)
	}

	buf, err := codectool.MarshalJSON(kind)
	if err != nil {
		return fmt.Errorf("BUG: marshal %#v to json failed: %v", kind, err)
	}

	key := s.kindKey(kind.Name)
	return s.cluster.Put(key, string(buf))
}

// DeleteAllKind deletes all custom data kinds with all data
func (s *Store) DeleteAllKinds() error {
	err := s.cluster.DeletePrefix(s.DataPrefix)
	if err != nil {
		return err
	}
	return s.cluster.DeletePrefix(s.KindPrefix)
}

// DeleteKind deletes a custom data kind
func (s *Store) DeleteKind(name string) error {
	kind, err := s.GetKind(name)
	if err != nil {
		return err
	}

	if kind == nil {
		return fmt.Errorf("%s not found", name)
	}

	if err := s.DeleteAllData(kind.Name); err != nil {
		return err
	}

	return s.cluster.Delete(s.kindKey(name))
}

func (s *Store) dataPrefix(kind string) string {
	return s.DataPrefix + kind + "/"
}

func (s *Store) dataKey(kind string, id string) string {
	return s.dataPrefix(kind) + id
}

func unmarshalData(yamlConfig []byte) (Data, error) {
	data := Data{}
	err := codectool.Unmarshal(yamlConfig, &data)
	if err != nil {
		return nil, fmt.Errorf("BUG: unmarshal %s to json failed: %v", string(yamlConfig), err)
	}
	return data, nil
}

// GetData gets custom data by its id
func (s *Store) GetData(kind string, id string) (Data, error) {
	kvs, err := s.cluster.GetRaw(s.dataKey(kind, id))
	if err != nil {
		return nil, err
	}

	if kvs == nil {
		return nil, nil
	}

	return unmarshalData(kvs.Value)
}

// DataLen returns the number of custom data of specified kind.
func (s *Store) DataLen(kind string) (int, error) {
	key := s.DataPrefix
	if kind != "" {
		key = s.dataPrefix(kind)
	}

	kvs, err := s.cluster.GetWithOp(key, cluster.OpPrefix, cluster.OpKeysOnly)
	if err != nil {
		return 0, err
	}
	return len(kvs), nil
}

// ListData lists custom data of specified kind.
// if kind is empty, it returns custom data of all kinds.
func (s *Store) ListData(kind string) ([]Data, error) {
	key := s.DataPrefix
	if kind != "" {
		key = s.dataPrefix(kind)
	}
	kvs, err := s.cluster.GetRawPrefix(key)
	if err != nil {
		return nil, err
	}

	results := make([]Data, 0, len(kvs))
	for _, v := range kvs {
		data, err := unmarshalData(v.Value)
		if err != nil {
			return nil, err
		}
		results = append(results, data)
	}

	return results, nil
}

// PutData creates or updates a custom data item
func (s *Store) PutData(kind string, data Data, update bool) (string, error) {
	k, err := s.GetKind(kind)
	if err != nil {
		return "", err
	}
	if k == nil {
		return "", fmt.Errorf("kind %s not found", kind)
	}

	id := k.dataID(data)
	if id == "" {
		return "", fmt.Errorf("data id is empty")
	}

	if len(k.JSONSchema) > 0 {
		schema := gojsonschema.NewGoLoader(k.JSONSchema)
		doc := gojsonschema.NewGoLoader(data)
		res, err := gojsonschema.Validate(schema, doc)
		if err != nil {
			return "", fmt.Errorf("error occurs during validation: %v", err)
		}
		if !res.Valid() {
			return "", fmt.Errorf("validation failed: %v", res.Errors())
		}
	}

	oldData, err := s.GetData(kind, id)
	if err != nil {
		return "", err
	}
	if update && (oldData == nil) {
		return "", fmt.Errorf("%s/%s not found", kind, id)
	}
	if (!update) && (oldData != nil) {
		return "", fmt.Errorf("%s/%s existed", kind, id)
	}

	buf, err := codectool.MarshalJSON(data)
	if err != nil {
		return "", fmt.Errorf("BUG: marshal %#v to json failed: %v", data, err)
	}

	key := s.dataKey(kind, id)
	err = s.cluster.Put(key, string(buf))
	if err != nil {
		return "", err
	}

	return id, nil
}

// BatchUpdateData updates multiple custom data in a transaction
func (s *Store) BatchUpdateData(kind string, del []string, update []Data) error {
	k, err := s.GetKind(kind)
	if err != nil {
		return err
	}
	if k == nil {
		return fmt.Errorf("kind %s not found", kind)
	}

	if len(update) > 0 && len(k.JSONSchema) > 0 {
		schema := gojsonschema.NewGoLoader(k.JSONSchema)
		for _, data := range update {
			doc := gojsonschema.NewGoLoader(data)
			res, err := gojsonschema.Validate(schema, doc)
			if err != nil {
				return fmt.Errorf("error occurs during validation: %v", err)
			}
			if !res.Valid() {
				return fmt.Errorf("validation failed: %v", res.Errors())
			}
		}
	}

	for _, data := range update {
		if k.dataID(data) == "" {
			return fmt.Errorf("data id is empty")
		}
	}

	return s.cluster.STM(func(stm concurrency.STM) error {
		for _, id := range del {
			key := s.dataKey(kind, id)
			stm.Del(key)
		}

		for _, data := range update {
			id := k.dataID(data)
			buf, err := codectool.MarshalJSON(data)
			if err != nil {
				return fmt.Errorf("BUG: marshal %#v to json failed: %v", data, err)
			}
			key := s.dataKey(kind, id)
			stm.Put(key, string(buf))
		}
		return nil
	})
}

// DeleteData deletes a custom data
func (s *Store) DeleteData(kind string, id string) error {
	data, err := s.GetData(kind, id)
	if err != nil {
		return err
	}

	if data == nil {
		return fmt.Errorf("%s not found", id)
	}

	return s.cluster.Delete(s.dataKey(kind, id))
}

// DeleteAllData deletes all custom data of kind 'kind'
func (s *Store) DeleteAllData(kind string) error {
	prefix := s.dataPrefix(kind)
	return s.cluster.DeletePrefix(prefix)
}

// Watch watches the data of custom data kind 'kind'
func (s *Store) Watch(ctx context.Context, kind string, onChange func([]Data)) error {
	syncer, err := s.cluster.Syncer(5 * time.Minute)
	if err != nil {
		return err
	}

	prefix := s.dataPrefix(kind)
	ch, err := syncer.SyncRawPrefix(prefix)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			syncer.Close()
			return nil
		case m := <-ch:
			data := make([]Data, 0, len(m))
			for _, v := range m {
				d, err := unmarshalData(v.Value)
				if err == nil {
					data = append(data, d)
				}
			}
			onChange(data)
		}
	}
}
