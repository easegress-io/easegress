/**
 * Created by g7tianyi on 2/21/17
 */

package common

import (
	"encoding/json"
)

func Json(obj interface{}) string {
	if b, err := json.Marshal(obj); err != nil {
		return ""
	} else {
		return string(b)
	}
}

func PrettyJson(obj interface{}) string {
	if b, err := json.MarshalIndent(obj, "", "  "); err != nil {
		return ""
	} else {
		return string(b)
	}
}
