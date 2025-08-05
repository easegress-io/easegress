package protocol

import "reflect"

type (
	RuleSpecification interface {
		// Name returns the name of the rule.
		Name() string
	}

	// RuleGroupSpec defines the specification for a WAF rule group.
	RuleGroupSpec struct {
		Name  string   `json:"name" jsonschema:"required"`
		Rules RuleSpec `json:"rules" jsonschema:"required"`
	}

	// RuleSpec defines a WAF rule.
	RuleSpec struct {
		SQLInjection *SQLInjectionSpec `json:"sqlInjection,omitempty"`
		Customs      *CustomsSpec      `json:"customRules,omitempty"`
	}

	// SQLInjectionSpec defines the specification for SQL injection detection.
	SQLInjectionSpec struct {
		Enabled bool `json:"enabled" jsonschema:"required"`
		// Additional fields can be added here for SQL injection detection configuration.
	}

	// CustomsSpec defines a custom WAF rule.
	CustomsSpec struct {
		CustomRules []CustomRuleSpec `json:"customRule" jsonschema:"required"`
	}

	// CustomRuleSpec defines the specification for a custom WAF rule.
	CustomRuleSpec struct {
		Enabled     bool   `json:"enabled" jsonschema:"required"`
		ModSecurity string `json:"modSecurity,omitempty"` // ModSecurity rule string.
		// Additional fields can be added here for custom rule configuration.
	}
)

func (sql *SQLInjectionSpec) Name() string {
	return "SQLInjection"
}

func (rule *CustomsSpec) Name() string {
	return "Customs"
}

func (rule *RuleSpec) GetSpec(name string) RuleSpecification {
	v := reflect.ValueOf(rule)
	field := v.FieldByName(name)
	if !field.IsValid() {
		return nil
	}
	if field.Kind() == reflect.Struct {
		if spec, ok := field.Interface().(RuleSpecification); ok {
			return spec
		}
		return nil
	}
	return nil
}
