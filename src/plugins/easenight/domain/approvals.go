/**
 * Created by g7tianyi on 13/08/2017
 */

package domain

type Approvals struct {
	Required bool     `json:"required"`
	Users    []string `json:"users,omitempty"`
}

func NewApprovals() *Approvals {
	return &Approvals{
		Required: false,
		Users:    make([]string, 0),
	}
}

func ParseApprovalsFromMap(m map[string]interface{}) *Approvals {
	approvals := NewApprovals()

	if val, ok := m["required"]; ok {
		if boolVal, ok := val.(bool); ok {
			approvals.Required = boolVal
		}
	}

	if val, ok := m["users"]; ok {
		if arrVal, ok := val.([]interface{}); ok {
			for _, arrV := range arrVal {
				if strVal, ok := arrV.(string); ok {
					approvals.Users = append(approvals.Users, strVal)
				}
			}
		}
	}

	return approvals
}
