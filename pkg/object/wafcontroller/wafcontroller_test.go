package wafcontroller

import (
	"testing"

	"github.com/megaease/easegress/v2/pkg/object/wafcontroller/protocol"
	r "github.com/megaease/easegress/v2/pkg/object/wafcontroller/rules"
)

func TestGenerateRules(t *testing.T) {
	spec := &Spec{
		RuleGroups: []*protocol.RuleGroupSpec{
			{
				Name: "testGroup",
				Rules: protocol.RuleSpec{
					Customs: &protocol.CustomsSpec{
						CustomRules: []protocol.CustomRuleSpec{
							{
								Enabled:     true,
								ModSecurity: "SecRule ARGS \"@rx ^(.*)$\" \"id:123,phase:2,pass,log,msg:'Test rule'\"",
							},
							{
								Enabled:     false,
								ModSecurity: "SecRule ARGS \"@rx ^(.*)$\" \"id:124,phase:2,pass,log,msg:'Disabled rule'\"",
							},
						},
					},
				},
			},
		},
	}

	rules := r.NewRules(spec.RuleGroups[0].Rules)
	if len(rules) != 1 {
		t.Errorf("Expected 1 rules, got %d", len(rules))
	}
	for _, rule := range rules {
		if rule.Name() == "Customs" {
			customRule := rule.(*r.Custom)
			if len(customRule.CustomRules) != 2 {
				t.Errorf("Expected 2 custom rules, got %d", len(customRule.CustomRules))
			}
			for _, cr := range customRule.CustomRules {
				if cr.Enabled && cr.ModSecurity == "" {
					t.Error("Enabled custom rule should have a ModSecurity string")
				}
			}
		} else {
			t.Errorf("Unexpected rule type: %s", rule.Name())
		}
	}
}
