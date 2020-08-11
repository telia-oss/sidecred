// Package inprocess implements a sidecred.SecretStore in memory, and can be used for tests.
package inprocess

import (
	"encoding/json"
	"fmt"

	"github.com/telia-oss/sidecred"
)

// New creates a new sidecred.SecretStore using an inprocess backend.
func New(options ...option) sidecred.SecretStore {
	s := &store{
		secrets:        make(map[string]string),
		secretTemplate: "{{ .Namespace }}.{{ .Name }}",
	}
	for _, optionFunc := range options {
		optionFunc(s)
	}
	return s
}

type option func(*store)

// WithSecretTemplate sets the path template when instanciating a new store.
func WithSecretTemplate(t string) option {
	return func(s *store) {
		s.secretTemplate = t
	}
}

type store struct {
	secrets        map[string]string
	secretTemplate string
}

// config that can be passed to the Configure method of this store.
type config struct {
	SecretTemplate string `json:"secret_template"`
}

// Type implements sidecred.SecretStore.
func (s *store) Type() sidecred.StoreType {
	return sidecred.Inprocess
}

// Write implements secretstore.SecretStore.
func (s *store) Write(namespace string, secret *sidecred.Credential, config json.RawMessage) (string, error) {
	c, err := s.parseConfig(config)
	if err != nil {
		return "", fmt.Errorf("parse config: %s", err)
	}
	path, err := sidecred.BuildSecretTemplate(c.SecretTemplate, namespace, secret.Name)
	if err != nil {
		return "", err
	}
	s.secrets[path] = secret.Value

	return path, nil
}

// Read implements secretstore.SecretStore.
func (s *store) Read(path string, _ json.RawMessage) (string, bool, error) {
	v, ok := s.secrets[path]
	if !ok {
		return "", false, nil
	}
	return v, true, nil
}

// Delete implements secretstore.SecretStore.
func (s *store) Delete(path string, _ json.RawMessage) error {
	delete(s.secrets, path)
	return nil
}

// parseConfig parses and validates the config.
func (s *store) parseConfig(raw json.RawMessage) (*config, error) {
	c := &config{}
	if err := sidecred.UnmarshalConfig(raw, &c); err != nil {
		return nil, err
	}
	if c.SecretTemplate == "" {
		c.SecretTemplate = s.secretTemplate
	}
	return c, nil
}
