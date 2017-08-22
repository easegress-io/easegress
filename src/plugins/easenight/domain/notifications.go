/**
 * Created by g7tianyi on 30/05/2017
 */

package domain

import (
	"plugins/easenight/common"
)

type Notifications struct {
	// TODO(g7tianyi): support verbose switch.
	// When in verbose mode, notifications'd be sent for every pastor host,
	// and be sent only when all pastor hosts are complete if otherwise.
	Slack    bool     `json:"slack"`
	Emails   []string `json:"emails,omitempty"`
	Webhooks []string `json:"webhooks,omitempty"`
}

func NewNotifications() *Notifications {
	return &Notifications{
		Slack:    false,
		Emails:   make([]string, 0),
		Webhooks: make([]string, 0),
	}
}

func ParseNotificationsFromMap(m map[string]interface{}) *Notifications {
	notifications := NewNotifications()

	if val, ok := m["slack"]; ok {
		notifications.Slack = common.ParseBoolValue(val)
	}

	if val, ok := m["emails"]; ok {
		notifications.Emails = common.ParseStringArray(val)
	}

	if val, ok := m["webhooks"]; ok {
		notifications.Webhooks = common.ParseStringArray(val)
	}

	return notifications
}
