package sidecred

import (
	"encoding/json"
	"time"
)

// StateBackend is implemented by things that know how to store sidecred.State.
type StateBackend interface {
	// Load state from the backend. If no state exists it should be created.
	Load(path string) (*State, error)

	// Save a state to the backend.
	Save(path string, state *State) error
}

// NewState returns a new sidecred.State.
func NewState() *State {
	return &State{}
}

// State is responsible for keeping track of when credentials need to be
// rotated because they are expired, the configuration has changed, or
// they have been deposed and need to clean up resources and secrets.
type State struct {
	Providers []*providerState `json:"providers,omitempty"`
	Stores    []*storeState    `json:"stores,omitempty"`
}

type providerState struct {
	Type      ProviderType `json:"type"`
	Resources []*Resource  `json:"resources"`
}

func (s *State) getProviderState(t ProviderType) (*providerState, bool) {
	for _, provider := range s.Providers {
		if provider.Type == t {
			return provider, true
		}
	}
	return nil, false
}

// newResource returns a new sidecred.Resource.
func newResource(request *CredentialRequest, expiration time.Time, metadata *Metadata) *Resource {
	return &Resource{
		ID:         request.Name,
		Config:     request.Config,
		Expiration: expiration,
		Deposed:    false,
		Metadata:   metadata,
		InUse:      true,
	}
}

// Resource represents a resource provisioned by a sidecred.Provider as
// part of creating the requested credentials.
type Resource struct {
	ID         string          `json:"id"`
	Expiration time.Time       `json:"expiration"`
	Deposed    bool            `json:"deposed"`
	Config     json.RawMessage `json:"config,omitempty"`
	Metadata   *Metadata       `json:"metadata,omitempty"`
	InUse      bool            `json:"-"`
}

// AddResource stores a resource state for the given provider. The provider
// will be added to state if it does not already exist. Any existing resources
// with the same ID will be marked as deposed.
func (s *State) AddResource(t ProviderType, resource *Resource) {
	var state *providerState
	for _, provider := range s.Providers {
		if provider.Type == t {
			state = provider
		}
	}
	if state == nil {
		state = &providerState{Type: t}
		s.Providers = append(s.Providers, state)
	}
	for i, res := range state.Resources {
		if res.ID == resource.ID {
			state.Resources[i].Deposed = true
		}
	}
	state.Resources = append(state.Resources, resource)
}

// GetResourcesByID returns all resources with the given ID from state, and also
// marks the resources as being in use.
func (s *State) GetResourcesByID(t ProviderType, id string) []*Resource {
	p, ok := s.getProviderState(t)
	if !ok {
		return nil
	}
	var resources []*Resource
	for _, r := range p.Resources {
		if r.ID == id {
			resources, r.InUse = append(resources, r), true
		}
	}
	return resources
}

// RemoveResource from the state.
func (s *State) RemoveResource(t ProviderType, resource *Resource) {
	state, ok := s.getProviderState(t)
	if !ok {
		return
	}
	for i, res := range state.Resources {
		if res.ID == resource.ID {
			state.Resources = append(state.Resources[:i], state.Resources[i+1:]...)
			break
		}
	}
}

type storeState struct {
	Type    StoreType `json:"type"`
	Secrets []*Secret `json:"secrets"`
}

// newSecret returns a sidecred.Secret for storing in state.
func newSecret(resourceID, path string, experation time.Time) *Secret {
	return &Secret{ResourceID: resourceID, Path: path, Expiration: experation}
}

// Secret is used to hold state about secrets stored in a secret backend.
type Secret struct {
	ResourceID string    `json:"resource_id"`
	Path       string    `json:"path"`
	Expiration time.Time `json:"expiration"`
}

func (s *State) getSecretStoreState(t StoreType) (*storeState, bool) {
	for _, store := range s.Stores {
		if store.Type == t {
			return store, true
		}
	}
	return nil, false
}

// AddSecret adds state for the specified sidecred.SecretStore. The
// store will be added to state if it does not already exist, and any
// existing state for the same secret path will be overwritten.
func (s *State) AddSecret(t StoreType, secret *Secret) {
	state, ok := s.getSecretStoreState(t)
	if !ok {
		state = &storeState{Type: t}
		s.Stores = append(s.Stores, state)
	}
	for i, sec := range state.Secrets {
		if sec.Path == secret.Path {
			state.Secrets[i] = secret
			return
		}
	}
	state.Secrets = append(state.Secrets, secret)
}

// ListOrphanedSecrets lists all secrets tied to missing resource
// IDs that should be considered orhpaned.
func (s *State) ListOrphanedSecrets(t StoreType) []*Secret {
	validResourceIDs := make(map[string]struct{})
	for _, p := range s.Providers {
		for _, r := range p.Resources {
			validResourceIDs[r.ID] = struct{}{}
		}
	}
	state, ok := s.getSecretStoreState(t)
	if !ok {
		return nil
	}
	var orphaned []*Secret
	for _, sec := range state.Secrets {
		if _, ok := validResourceIDs[sec.ResourceID]; ok {
			continue
		}
		orphaned = append(orphaned, sec)
	}
	return orphaned
}

// RemoveSecret from the state.
func (s *State) RemoveSecret(t StoreType, secret *Secret) {
	state, ok := s.getSecretStoreState(t)
	if !ok {
		return
	}
	for i, sec := range state.Secrets {
		if sec.Path == secret.Path {
			state.Secrets = append(state.Secrets[:i], state.Secrets[i+1:]...)
			break
		}
	}
}
