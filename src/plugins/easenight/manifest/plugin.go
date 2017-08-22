/**
 * Created by g7tianyi on 14/08/2017
 */

package manifest

import (
	"encoding/json"

	"plugins/easenight/common"
	"plugins/easenight/domain"
)

type Plugin struct {
	Props map[string]interface{}
}

func NewPlugin() *Plugin {
	return &Plugin{make(map[string]interface{})}
}

func (p *Plugin) GetName() string {
	return common.ParseStringValue(p.Props[NAME])
}

func (p *Plugin) GetProps() map[string]interface{} {
	return p.Props
}

func (p *Plugin) MarshalJSON() ([]byte, error) {
	m := make(map[string]interface{})
	for key, val := range p.Props {
		m[key] = val
	}
	return json.Marshal(m)
}

func (p *Plugin) UnmarshalJSON(data []byte) error {
	err := json.Unmarshal(data, &p.Props)
	if err != nil {
		return err
	}
	return nil
}

func (p *Plugin) MarshalYAML() (yamlString string, err error) {
	m := make(map[string]map[string]interface{})
	yamlString, err = common.Marshal(m)
	return
}

func (p *Plugin) UnmarshalYAML(yamlStr string) error {
	err := common.Unmarshal(yamlStr, &p.Props)
	if err != nil {
		return err
	}
	return nil
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func (p *Plugin) GetEnv() string {
	return common.ParseStringValueWithDefault(p.Props[ENV], DEFAULT_ENVIRONMENT)
}

func (p *Plugin) GetShell() string {
	return common.ParseStringValue(p.Props[SHELL])
}

func (p *Plugin) GetScript() string {
	return common.ParseStringValue(p.Props[SCRIPT])
}

func (p *Plugin) GetTimeout() int {
	return common.ParseIntValue(p.Props[TIMEOUT])
}

func (p *Plugin) GetVariables() map[string]interface{} {
	return common.ParseMap(p.Props[VARIABLES])
}

func (p *Plugin) GetApprovals() *domain.Approvals {
	val := p.Props[APPROVALS]
	if val == nil {
		return nil
	}
	if ret, ok := val.(domain.Approvals); !ok {
		return domain.NewApprovals()
	} else if m, ok := val.(map[string]interface{}); ok {
		return domain.ParseApprovalsFromMap(m)
	} else {
		return &ret
	}
}

func (p *Plugin) GetNotifications() *domain.Notifications {
	val := p.Props[NOTIFICATIONS]
	if val == nil {
		return nil
	}
	if ret, ok := val.(domain.Notifications); ok {
		return &ret
	} else if m, ok := val.(map[string]interface{}); ok {
		return domain.ParseNotificationsFromMap(m)
	}
	return domain.NewNotifications()
}

func (p *Plugin) GetDockerRegistry() string {
	return common.ParseStringValue(p.Props[DOCKER_REGISTRY])
}

func (p *Plugin) GetDockerFile() string {
	return common.ParseStringValue(p.Props[DOCKER_FILE])
}

func (p *Plugin) GetImageName() string {
	return common.ParseStringValue(p.Props[IMAGE_NAME])
}

func (p *Plugin) GetPushImage() bool {
	return common.ParseBoolValue(p.Props[PUSH_IMAGE])
}

func (p *Plugin) GetEaseStack() *domain.EaseStack {
	value := p.Props[EASE_STACK]
	if value == nil {
		return nil
	}
	if easeStack, ok := value.(domain.EaseStack); ok {
		return &easeStack
	}

	m := common.ParseMap(value)
	if m == nil {
		return nil
	}

	easeStack := new(domain.EaseStack)
	easeStack.Start = common.ParseStringValue(m[EASE_STACK_START])
	easeStack.Stop = common.ParseStringValue(m[EASE_STACK_STOP])
	easeStack.ApplyConfig = common.ParseStringValue(m[EASE_STACK_APPLY_CONFIG])
	easeStack.HealthCheck = common.ParseStringValue(m[EASE_STACK_HEALTH_CHECK])
	return easeStack
}

func (p *Plugin) GetAppLocation() string {
	return common.ParseStringValue(p.Props[APP_LOCATION])
}

func (p *Plugin) GetAppName() string {
	return common.ParseStringValue(p.Props[APP_NAME])
}
