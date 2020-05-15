// Package random implements a sidecred.Provider for random strings, and can be used for tests.
package random

import (
	"math/rand"
	"time"

	"github.com/telia-oss/sidecred"
)

// RequestConfig ...
type RequestConfig struct {
	Length int `json:"length"`
}

// New returns a new sidecred.Provider for random strings.
func New(seed int64, options ...option) sidecred.Provider {
	p := &provider{
		generator:        rand.New(rand.NewSource(seed)),
		chars:            "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%&*",
		rotationInterval: time.Duration(time.Hour * 24 * 30),
	}
	for _, optionFunc := range options {
		optionFunc(p)
	}
	return p
}

type option func(*provider)

// WithRotationInterval sets the interval at which the random string should be rotated.
func WithRotationInterval(duration time.Duration) option {
	return func(p *provider) {
		p.rotationInterval = duration
	}
}

type provider struct {
	generator        *rand.Rand
	chars            string
	rotationInterval time.Duration
}

// Type implements sidecred.Provider.
func (p *provider) Type() sidecred.ProviderType {
	return sidecred.Random
}

// Provide implements sidecred.Provider.
func (p *provider) Create(request *sidecred.Request) ([]*sidecred.Credential, *sidecred.Metadata, error) {
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
			Expiration:  time.Now().Add(p.rotationInterval).UTC(),
		},
	}, nil, nil
}

// Destroy implements sidecred.Provider.
func (p *provider) Destroy(resource *sidecred.Resource) error {
	return nil
}
