// Package random implements a sidecred.Provider for random strings, and can be used for tests.
package random

import (
	"math/rand"
	"time"

	"github.com/telia-oss/sidecred"
)

var _ sidecred.Validatable = &RequestConfig{}

// RequestConfig ...
type RequestConfig struct {
	Length int `json:"length"`
}

// Validate implements sidecred.Validatable.
func (c *RequestConfig) Validate() error {
	return nil
}

// New returns a new sidecred.Provider for random strings.
func New(seed int64, opts Options) sidecred.Provider {
	if opts.RotationInterval == 0 {
		opts.RotationInterval = time.Hour * 24 * 7
	}
	return &provider{
		generator: rand.New(rand.NewSource(seed)),
		chars:     "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%&*",
		opts:      opts,
	}
}

// Options for the provider.
type Options struct {
	// RotationInterval specifies the interval at which the random string should be rotated.
	RotationInterval time.Duration
}

type provider struct {
	generator *rand.Rand
	chars     string
	opts      Options
}

// Type implements sidecred.Provider.
func (p *provider) Type() sidecred.ProviderType {
	return sidecred.Random
}

// Create implements sidecred.Provider.
func (p *provider) Create(request *sidecred.CredentialRequest) ([]*sidecred.Credential, *sidecred.Metadata, error) {
	var c RequestConfig
	if err := request.UnmarshalConfig(&c); err != nil {
		return nil, nil, err
	}
	b := make([]byte, c.Length)
	for i := range b {
		b[i] = p.chars[p.generator.Intn(len(p.chars))]
	}
	return []*sidecred.Credential{
		{
			Name:        request.Name,
			Value:       string(b),
			Description: "Random generated secret managed by Sidecred.",
			Expiration:  time.Now().Add(p.opts.RotationInterval).UTC(),
		},
	}, nil, nil
}

// Destroy implements sidecred.Provider.
func (p *provider) Destroy(_ *sidecred.Resource) error {
	return nil
}
