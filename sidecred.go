package sidecred

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
	"time"

	"go.uber.org/zap"
)

// Request is the root datastructure used to request credentials in Sidecred.
type Request struct {
	// Type identifies the type of credential (and provider) for a request.
	Type CredentialType `json:"type"`

	// Name is an indentifier that can be used for naming resources and
	// credentials created by a sidecred.Provider. The exact usage for
	// name is up to the individual provider.
	Name string `json:"name"`

	// Config holds the specific configuration for the requested credential
	// type, and must be deserialized by the provider when Create is called.
	Config json.RawMessage `json:"config"`
}

// hasValidCredentials returns true if there are already valid credentials
// for the request. This is determined by the last resource state.
func (r *Request) hasValidCredentials(resource *Resource) bool {
	if resource.Deposed {
		return false
	}
	if r.Name != resource.ID {
		return false
	}
	if !bytes.Equal(r.Config, resource.Config) {
		return false
	}
	if resource.Expiration.Before(time.Now()) {
		return false
	}
	return true
}

// UnmarshalConfig is a convenience method for unmarshalling the JSON config into
// a config structure for a sidecred.Provider. When no config has been passed in
// the request, no operation is performed by this function.
func (r *Request) UnmarshalConfig(target interface{}) error {
	if len(r.Config) == 0 {
		return nil
	}
	if err := json.Unmarshal(r.Config, target); err != nil {
		return fmt.Errorf("%s request: unmarshal: %s", r.Type, err)
	}
	return nil
}

// CredentialType ...
type CredentialType string

// Enumeration of known credential types.
const (
	Randomized        CredentialType = "random"
	AWSSTS            CredentialType = "aws:sts"
	GithubDeployKey   CredentialType = "github:deploy-key"
	GithubAccessToken CredentialType = "github:access-token"
)

// Provider returns the sidecred.ProviderType for the credential.
func (c CredentialType) Provider() ProviderType {
	switch c {
	case Randomized:
		return Random
	case AWSSTS:
		return AWS
	case GithubDeployKey, GithubAccessToken:
		return Github
	}
	return ProviderType(c)
}

// Enumeration of known provider types.
const (
	Random ProviderType = "random"
	AWS    ProviderType = "aws"
	Github ProviderType = "github"
)

// ProviderType ...
type ProviderType string

// Provider is the interface that has to be satisfied by credential providers.
type Provider interface {
	// Type returns the provider type.
	Type() ProviderType

	// Create the requested credentials. Any sidecred.Resource
	// returned will be stored in state and used to determine
	// when credentials need to be rotated.
	Create(request *Request) ([]*Credential, *Metadata, error)

	// Destroy the specified resource. This is scheduled if
	// a resource in the state has expired. For providers that
	// are not stateful this should be a no-op.
	Destroy(resource *Resource) error
}

// Metadata allows providers to pass additional information to be
// stored in the sidecred.ResourceState after successfully creating
// credentials.
type Metadata map[string]string

// Credential is a key/value pair returned by a sidecred.Provider.
type Credential struct {
	// Name is the identifier for the credential.
	Name string `json:"name,omitempty"`

	// Value is the credential value (typically a secret).
	Value string `json:"-"`

	// Description returns a short description of the credential.
	Description string `json:"-"`

	// Expiration is the time at which the credential will have expired.
	Expiration time.Time `json:"expiration"`
}

// Enumeration of known backends.
const (
	Inprocess      StoreType = "inprocess"
	SecretsManager StoreType = "secretsmanager"
	SSM            StoreType = "ssm"
)

// StoreType ...
type StoreType string

// SecretStore is implemented by store backends for secrets.
type SecretStore interface {
	// Type returns the store type.
	Type() StoreType

	// Write a sidecred.Credential to the secret store.
	Write(namespace string, secret *Credential) (string, error)

	// Read the specified secret by reference.
	Read(path string) (string, bool, error)

	// Delete the specified secret. Should not return an error
	// if the secret does not exist or has already been deleted.
	Delete(path string) error
}

// BuildSecretPath is a convenience function for building path templates.
func BuildSecretPath(pathTemplate, namespace, name string) (string, error) {
	t, err := template.New("path").Option("missingkey=error").Parse(pathTemplate)
	if err != nil {
		return "", err
	}

	var p strings.Builder

	if err = t.Execute(&p, struct {
		Namespace string
		Name      string
	}{
		Namespace: namespace,
		Name:      name,
	}); err != nil {
		return "", err
	}

	return p.String(), nil
}

// New returns a new instance of sidecred.Sidecred with the desired configuration.
func New(providers []Provider, store SecretStore, logger *zap.Logger) (*Sidecred, error) {
	s := &Sidecred{
		providers: make(map[ProviderType]Provider, len(providers)),
		store:     store,
		logger:    logger,
	}
	for _, p := range providers {
		s.providers[p.Type()] = p
	}
	return s, nil
}

// Sidecred is the underlying datastructure for the service.
type Sidecred struct {
	providers map[ProviderType]Provider
	store     SecretStore
	logger    *zap.Logger
}

// Process a single sidecred.Request.
func (s *Sidecred) Process(namespace string, requests []*Request, state *State) error {
	log := s.logger.With(zap.String("namespace", namespace))
	log.Info("starting sidecred", zap.Int("requests", len(requests)))

Loop:
	for _, r := range requests {
		log := log.With(zap.String("type", string(r.Type)))
		if r.Name == "" {
			log.Warn("missing name in request")
			continue Loop
		}
		p, ok := s.providers[r.Type.Provider()]
		if !ok {
			log.Warn("provider not configured")
			continue Loop
		}
		log.Info("processing request", zap.String("name", r.Name))

		for _, resource := range state.GetResourcesByID(p.Type(), r.Name) {
			if r.hasValidCredentials(resource) {
				log.Info("found existing credentials", zap.String("name", r.Name))
				continue Loop
			}
		}

		creds, metadata, err := p.Create(r)
		if err != nil {
			log.Error("failed to provide credentials", zap.Error(err))
			continue Loop
		}
		if len(creds) == 0 {
			log.Error("no credentials returned by provider")
			continue Loop
		}
		state.AddResource(p.Type(), newResource(r, creds[0].Expiration, metadata))
		log.Info("created new credentials", zap.Int("count", len(creds)))

		for _, c := range creds {
			path, err := s.store.Write(namespace, c)
			if err != nil {
				log.Error("store credential", zap.String("name", c.Name), zap.Error(err))
				continue
			}
			state.AddSecret(s.store.Type(), newSecret(r.Name, path, c.Expiration))
			log.Debug("stored credential", zap.String("path", path))
		}
		log.Info("done processing")
	}

	for _, ps := range state.Providers {
		// Reverse loop to handle index changes due to deleting items in the underlying array.
		for i := len(ps.Resources) - 1; i >= 0; i-- {
			r := ps.Resources[i]
			if r.InUse && !r.Deposed && r.Expiration.After(time.Now()) {
				continue
			}
			provider, ok := s.providers[ps.Type]
			if !ok {
				log.Debug("missing provider for expired resource", zap.String("type", string(ps.Type)))
				continue
			}
			log := s.logger.With(
				zap.String("type", string(ps.Type)),
				zap.String("id", r.ID),
			)
			log.Info("destroying expired resource")
			if err := provider.Destroy(r); err != nil {
				log.Error("destroy resource", zap.Error(err))
			}
			state.RemoveResource(provider.Type(), r)
		}
	}

	orphans := state.ListOrphanedSecrets(s.store.Type())
	for i := len(orphans) - 1; i >= 0; i-- {
		r := orphans[i]
		log.Info("deleting orphaned secret", zap.String("path", r.Path))
		if err := s.store.Delete(r.Path); err != nil {
			log.Error("delete secret", zap.String("path", r.Path), zap.Error(err))
		}
		state.RemoveSecret(s.store.Type(), r)
	}

	return nil
}
