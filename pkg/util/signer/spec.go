package signer

import "time"

// Spec defines the configuration of a Signer
type Spec struct {
	Literal         *Literal          `yaml:"literial,omitempty" json:"literial,omitempty" jsonschema:"omitempty"`
	HeaderHoisting  *HeaderHoisting   `yaml:"headerHoisting,omitempty" json:"headerHoisting,omitempty" jsonschema:"omitempty"`
	IgnoredHeaders  []string          `yaml:"ignoredHeaders" json:"ignoredHeaders" jsonschema:"omitempty,uniqueItems=true"`
	ExcludeBody     bool              `yaml:"excludeBody" json:"excludeBody" jsonschema:"omitempty"`
	TTL             string            `yaml:"ttl" json:"ttl" jsonschema:"omitempty,format=duration"`
	AccessKeyID     string            `yaml:"accessKeyId" json:"accessKeyId" jsonschema:"omitempty"`
	AccessKeySecret string            `yaml:"accessKeySecret" json:"accessKeySecret" jsonschema:"omitempty"`
	AccessKeys      map[string]string `yaml:"accessKeys" json:"accessKeys" jsonschema:"omitempty"`
	// TODO: AccessKeys is used as an internal access key stroe, but an external store is also needed
}

type idSecretMap map[string]string

func (m idSecretMap) GetSecret(id string) (string, bool) {
	s, ok := m[id]
	return s, ok
}

// CreateFromSpec create a Signer from configuration
func CreateFromSpec(spec *Spec) *Signer {
	signer := New()

	signer.SetCredential(spec.AccessKeyID, spec.AccessKeySecret)

	if spec.Literal != nil {
		signer.SetLiteral(spec.Literal)
	}

	if spec.HeaderHoisting != nil {
		signer.SetHeaderHoisting(spec.HeaderHoisting)
	}

	signer.IgnoreHeader(spec.IgnoredHeaders...)
	signer.ExcludeBody(spec.ExcludeBody)

	if ttl, e := time.ParseDuration(spec.TTL); e == nil {
		signer.SetTTL(ttl)
	}

	if len(spec.AccessKeys) > 0 {
		signer.SetAccessKeyStore(idSecretMap(spec.AccessKeys))
	}
	return signer
}
