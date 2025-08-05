package rules

import (
	"reflect"

	"github.com/megaease/easegress/v2/pkg/object/wafcontroller/protocol"
)

type (
	Custom protocol.CustomsSpec
)

func (rule *Custom) Name() string {
	return "Customs"
}

func (rule *Custom) Type() string {
	return "Customs"
}

func (rule *Custom) Spec() protocol.RuleSpecification {
	return rule
}

func (rule *Custom) Handle() *WAFResponse {
	// TODO: Custom rule handling logic should be implemented here.
	for _, spec := range rule.CustomRules {
		if spec.Enabled {
			// Process the ModSecurity rule string.
			// This is where you would integrate with ModSecurity or similar logic.
			// For now, we just return a placeholder response.
			return &WAFResponse{
				Message: "Custom rule matched: " + spec.ModSecurity,
				Result:  ResultBlocked,
			}
		}
	}
	return &WAFResponse{
		Message: "Custom rule handling not implemented",
		Result:  ResultOk,
	}
}

func (rule *Custom) init(ruleSpec protocol.RuleSpecification) {
	rule.CustomRules = ruleSpec.(*protocol.CustomsSpec).CustomRules
	if len(rule.CustomRules) == 0 {
		rule.CustomRules = []protocol.CustomRuleSpec{}
	}
}

func init() {
	RuleTypeRegistry["Customs"] = reflect.TypeOf(Custom{})
}
