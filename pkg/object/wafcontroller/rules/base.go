package rules

import "github.com/megaease/easegress/v2/pkg/object/wafcontroller/protocol"

type (
	// Rule defines a WAF rule.
	Rule interface {
		Name() string
		Type() string
		Spec() protocol.RuleSpecification
		Handle()
	}
)
