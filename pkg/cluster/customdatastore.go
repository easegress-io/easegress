/*
 * Copyright (c) 2017, MegaEase
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

package cluster

import (
	"context"
	"fmt"
	"time"

	"github.com/megaease/easegress/pkg/util/dynamicobject"
	"github.com/xeipuuv/gojsonschema"
	"go.etcd.io/etcd/client/v3/concurrency"
	yaml "gopkg.in/yaml.v2"
)

// CustomData represents a custom data
type CustomData dynamicobject.DynamicObject

// CustomDataKind defines the spec of a custom data kind
type CustomDataKind struct {
	// Name is the name of the CustomDataKind
	Name string `yaml:"name" jsonschema:"required"`
	// IDField is a field name of CustomData of this kind, this field is the ID
	// of the data, that's unique among the same kind, the default value is 'name'.
	IDField string `yaml:"idField" jsonschema:"omitempty"`
	// JSONSchema is JSON schema to validate a CustomData of this kind
	JSONSchema dynamicobject.DynamicObject `yaml:"jsonSchema" jsonschema:"omitempty"`
}

func (cdk *CustomDataKind) dataID(data CustomData) string {
	var id string
	if cdk.IDField == "" {
		id, _ = data["name"].(string)
	} else {
		id, _ = data[cdk.IDField].(string)
	}
	return id
}

// CustomDataStore defines the storage for custom data
type CustomDataStore struct {
	cluster    Cluster
	KindPrefix string
	DataPrefix string
}

// NewCustomDataStore creates a new custom data store
func NewCustomDataStore(cls Cluster, kindPrefix string, dataPrefix string) *CustomDataStore {
	return &CustomDataStore{
		cluster:    cls,
		KindPrefix: kindPrefix,
		DataPrefix: dataPrefix,
	}
}

func unmarshalCustomDataKind(in []byte) (*CustomDataKind, error) {
	kind := &CustomDataKind{}
	err := yaml.Unmarshal(in, kind)
	if err != nil {
		return nil, fmt.Errorf("BUG: unmarshal %s to yaml failed: %v", string(in), err)
	}
	return kind, nil
}

func (cds *CustomDataStore) kindKey(name string) string {
	return cds.KindPrefix + name
}

// GetKind gets custom data kind by its name
func (cds *CustomDataStore) GetKind(name string) (*CustomDataKind, error) {
	kvs, err := cds.cluster.GetRaw(cds.kindKey(name))
	if err != nil {
		return nil, err
	}

	if kvs == nil {
		return nil, nil
	}

	return unmarshalCustomDataKind(kvs.Value)
}

// ListKinds lists custom data kinds
func (cds *CustomDataStore) ListKinds() ([]*CustomDataKind, error) {
	kvs, err := cds.cluster.GetRawPrefix(cds.KindPrefix)
	if err != nil {
		return nil, err
	}

	kinds := make([]*CustomDataKind, 0, len(kvs))
	for _, v := range kvs {
		kind, err := unmarshalCustomDataKind(v.Value)
		if err != nil {
			return nil, err
		}
		kinds = append(kinds, kind)
	}

	return kinds, nil
}

func (cds *CustomDataStore) saveKind(kind *CustomDataKind, update bool) error {
	if len(kind.JSONSchema) > 0 {
		sl := gojsonschema.NewGoLoader(kind.JSONSchema)
		if _, err := gojsonschema.NewSchema(sl); err != nil {
			return fmt.Errorf("invalid JSONSchema: %s", err.Error())
		}
	}

	oldKind, err := cds.GetKind(kind.Name)
	if err != nil {
		return err
	}

	if update && (oldKind == nil) {
		return fmt.Errorf("%s not found", kind.Name)
	}
	if (!update) && (oldKind != nil) {
		return fmt.Errorf("%s existed", kind.Name)
	}

	buf, err := yaml.Marshal(kind)
	if err != nil {
		return fmt.Errorf("BUG: marshal %#v to yaml failed: %v", kind, err)
	}

	key := cds.kindKey(kind.Name)
	return cds.cluster.Put(key, string(buf))
}

// CreateKind creates a custom data kind
func (cds *CustomDataStore) CreateKind(kind *CustomDataKind) error {
	return cds.saveKind(kind, false)
}

// UpdateKind updates a custom data kind
func (cds *CustomDataStore) UpdateKind(kind *CustomDataKind) error {
	return cds.saveKind(kind, true)
}

// DeleteKind deletes a custom data kind
func (cds *CustomDataStore) DeleteKind(name string) error {
	kind, err := cds.GetKind(name)
	if err != nil {
		return err
	}

	if kind == nil {
		return fmt.Errorf("%s not found", name)
	}

	return cds.cluster.Delete(cds.kindKey(name))
	// TODO: remove custom data?
}

func (cds *CustomDataStore) dataPrefix(kind string) string {
	return cds.DataPrefix + kind + "/"
}

func (cds *CustomDataStore) dataKey(kind string, id string) string {
	return cds.dataPrefix(kind) + id
}

func unmarshalCustomData(in []byte) (CustomData, error) {
	data := CustomData{}
	err := yaml.Unmarshal(in, &data)
	if err != nil {
		return nil, fmt.Errorf("BUG: unmarshal %s to yaml failed: %v", string(in), err)
	}
	return data, nil
}

// GetData gets custom data by its id
func (cds *CustomDataStore) GetData(kind string, id string) (CustomData, error) {
	kvs, err := cds.cluster.GetRaw(cds.dataKey(kind, id))
	if err != nil {
		return nil, err
	}

	if kvs == nil {
		return nil, nil
	}

	return unmarshalCustomData(kvs.Value)
}

// ListData lists custom data of specified kind.
// if kind is empty, it returns custom data of all kinds.
func (cds *CustomDataStore) ListData(kind string) ([]CustomData, error) {
	key := cds.DataPrefix
	if kind != "" {
		key = cds.dataPrefix(kind)
	}
	kvs, err := cds.cluster.GetRawPrefix(key)
	if err != nil {
		return nil, err
	}

	results := make([]CustomData, 0, len(kvs))
	for _, v := range kvs {
		data, err := unmarshalCustomData(v.Value)
		if err != nil {
			return nil, err
		}
		results = append(results, data)
	}

	return results, nil
}

func (cds *CustomDataStore) saveData(kind string, data CustomData, update bool) (string, error) {
	k, err := cds.GetKind(kind)
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

	oldData, err := cds.GetData(kind, id)
	if err != nil {
		return "", err
	}
	if update && (oldData == nil) {
		return "", fmt.Errorf("%s/%s not found", kind, id)
	}
	if (!update) && (oldData != nil) {
		return "", fmt.Errorf("%s/%s existed", kind, id)
	}

	buf, err := yaml.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("BUG: marshal %#v to yaml failed: %v", data, err)
	}

	key := cds.dataKey(kind, id)
	err = cds.cluster.Put(key, string(buf))
	if err != nil {
		return "", err
	}

	return id, nil
}

// CreateData creates a custom data
func (cds *CustomDataStore) CreateData(kind string, data CustomData) (string, error) {
	return cds.saveData(kind, data, false)
}

// UpdateData updates a custom data
func (cds *CustomDataStore) UpdateData(kind string, data CustomData) (string, error) {
	return cds.saveData(kind, data, true)
}

// BatchUpdateData updates multiple custom data in a transaction
func (cds *CustomDataStore) BatchUpdateData(kind string, del []string, update []CustomData) error {
	k, err := cds.GetKind(kind)
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

	return cds.cluster.STM(func(s concurrency.STM) error {
		for _, id := range del {
			key := cds.dataKey(kind, id)
			s.Del(key)
		}

		for _, data := range update {
			id := k.dataID(data)
			buf, err := yaml.Marshal(data)
			if err != nil {
				return fmt.Errorf("BUG: marshal %#v to yaml failed: %v", data, err)
			}
			key := cds.dataKey(kind, id)
			s.Put(key, string(buf))
		}
		return nil
	})
}

// DeleteData deletes a custom data
func (cds *CustomDataStore) DeleteData(kind string, id string) error {
	data, err := cds.GetData(kind, id)
	if err != nil {
		return err
	}

	if data == nil {
		return fmt.Errorf("%s not found", id)
	}

	return cds.cluster.Delete(cds.dataKey(kind, id))
}

// DeleteAllData deletes all custom data of kind 'kind'
func (cds *CustomDataStore) DeleteAllData(kind string) error {
	prefix := cds.dataPrefix(kind)
	return cds.cluster.DeletePrefix(prefix)
}

// Watch watches the data of custom data kind 'kind'
func (cds *CustomDataStore) Watch(ctx context.Context, kind string, onChange func([]CustomData)) error {
	syncer, err := cds.cluster.Syncer(5 * time.Minute)
	if err != nil {
		return err
	}

	prefix := cds.dataPrefix(kind)
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
			data := make([]CustomData, 0, len(m))
			for _, v := range m {
				d, err := unmarshalCustomData(v.Value)
				if err == nil {
					data = append(data, d)
				}
			}
			onChange(data)
		}
	}
}
