/**
 * Created by g7tianyi on 15/08/2017
 */

package store

import (
	"encoding/json"
	"fmt"

	yaml "plugins/easenight/common"
)

type DataStore interface {
	fmt.Stringer
	json.Marshaler
	json.Unmarshaler
	yaml.Marshaler
	yaml.Unmarshaler

	Keys() []string
	// Values() []interface{}
	Put(key string, val interface{})
	HasValue(key string) bool
	IntValue(key string) int
	StringValue(key string) string
	StringArrayValue(key string) []string
	ObjectValue(key string) interface{}
}
