package rules

import (
	"github.com/megaease/easegress/v2/pkg/object/wafcontroller/protocol"
	"reflect"
)

type (
	// Rule defines a WAF rule.
	Rule interface {
		Name() string
		Type() string
		Spec() protocol.RuleSpecification
		Handle() *WAFResponse

		init(ruleSpec protocol.RuleSpecification)
	}

	// WAFResponse defines the response structure for WAF rules.
	WAFResponse struct {
		Message string            `json:"message,omitempty"`
		Result  WAFResponseResult `json:"result,omitempty"`
	}

	WAFResponseResult string
)

const (
	ResultOk      WAFResponseResult = "OK"
	ResultBlocked WAFResponseResult = "Blocked"
)

var RuleTypeRegistry = map[string]reflect.Type{}

func NewRules(ruleSpec protocol.RuleSpec) []Rule {
	v := reflect.ValueOf(ruleSpec)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	t := v.Type()
	rules := make([]Rule, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		field := v.Field(i)
		if field.Kind() == reflect.Ptr && !field.IsNil() {
			name := t.Field(i).Name
			if ruleType, ok := RuleTypeRegistry[name]; ok {
				rule := reflect.New(ruleType).Interface().(Rule)
				rule.init(field.Interface().(protocol.RuleSpecification))
				rules = append(rules, rule)
			}
		}
	}
	return rules
}
