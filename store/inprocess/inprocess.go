// Package inprocess implements a sidecred.SecretStore in memory, and can be used for tests.
package inprocess

import (
	"github.com/telia-oss/sidecred"
)

// New creates a new sidecred.SecretStore using an inprocess backend.
func New(options ...option) sidecred.SecretStore {
	s := &store{
		secrets:      make(map[string]string),
		pathTemplate: "{{ .Namespace }}.{{ .Name }}",
	}
	for _, optionFunc := range options {
		optionFunc(s)
	}
	return s
}

type option func(*store)

// WithPathTemplate sets the path template when instanciating a new store.
func WithPathTemplate(t string) option {
	return func(s *store) {
		s.pathTemplate = t
	}
}

type store struct {
	secrets      map[string]string
	pathTemplate string
}

// Type implements sidecred.SecretStore.
func (s *store) Type() sidecred.StoreType {
	return sidecred.Inprocess
}

// Write implements secretstore.SecretStore.
func (s *store) Write(namespace string, secret *sidecred.Credential) (string, error) {
	path, err := sidecred.BuildSecretPath(s.pathTemplate, namespace, secret.Name)
	if err != nil {
		return "", err
	}
	s.secrets[path] = secret.Value

	return path, nil
}

// Read implements secretstore.SecretStore.
func (s *store) Read(path string) (string, bool, error) {
	v, ok := s.secrets[path]
	if !ok {
		return "", false, nil
	}
	return v, true, nil
}

// Delete implements secretstore.SecretStore.
func (s *store) Delete(path string) error {
	delete(s.secrets, path)
	return nil
}
