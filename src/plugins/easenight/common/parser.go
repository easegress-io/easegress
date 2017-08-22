/**
 * Created by g7tianyi on 14/08/2017
 */

package common

func ParseStringValue(rawVal interface{}) string {
	return ParseStringValueWithDefault(rawVal, "")
}

func ParseStringValueWithDefault(rawVal interface{}, defaultValue string) string {
	if rawVal == nil {
		return defaultValue
	}
	if strVal, ok := rawVal.(string); ok {
		return strVal
	} else {
		return defaultValue
	}
}

func ParseBoolValue(rawVal interface{}) bool {
	return ParseBoolValueWithDefault(rawVal, false)
}

func ParseBoolValueWithDefault(rawVal interface{}, defaultValue bool) bool {
	if rawVal == nil {
		return defaultValue
	}
	if boolVal, ok := rawVal.(bool); ok {
		return boolVal
	} else {
		return defaultValue
	}
}

func ParseIntValue(rawVal interface{}) int {
	return ParseIntValueWithDefault(rawVal, 0)
}

func ParseIntValueWithDefault(rawVal interface{}, defaultValue int) int {
	if rawVal == nil {
		return defaultValue
	}
	if intVal, ok := rawVal.(int); ok {
		return intVal
	} else {
		return defaultValue
	}
}

func ParseFloat32Value(rawVal interface{}) float32 {
	return ParseFloat32ValueWithDefault(rawVal, .0)
}

func ParseFloat32ValueWithDefault(rawVal interface{}, defaultValue float32) float32 {
	if rawVal == nil {
		return defaultValue
	}
	if float32Val, ok := rawVal.(float32); ok {
		return float32Val
	} else {
		return defaultValue
	}
}

func ParseStringArray(rawVal interface{}) (ret []string) {
	if rawVal == nil {
		return
	}
	if arrVal, ok := rawVal.([]interface{}); ok {
		for _, arrV := range arrVal {
			if strVal, ok := arrV.(string); ok {
				ret = append(ret, strVal)
			}
		}
	}
	return
}

func ParseMap(rawVal interface{}) map[string]interface{} {
	if rawVal == nil {
		return nil
	}
	if m, ok := rawVal.(map[string]interface{}); ok {
		return m
	}
	return nil
}
