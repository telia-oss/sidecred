package sidecred

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
	"time"

	"go.uber.org/zap"
)

// CredentialRequest is the root datastructure used to request credentials in Sidecred.
type CredentialRequest struct {
	// Type identifies the type of credential (and provider) for a request.
	Type CredentialType `json:"type"`

	// Name is an indentifier that can be used for naming resources and
	// credentials created by a sidecred.Provider. The exact usage for
	// name is up to the individual provider.
	Name string `json:"name"`

	// Rotation is an override for the default rotation window
	// measured in seconds.
	// This will aid in cases where we want to be more granular
	// for possibly longer running authentications or processes.
	//
	// Needs to be a int because JSON unmarshalling does not support Duration
	RotationWindow int `json:"rotation_window"`

	// Config holds the specific configuration for the requested credential
	// type, and must be deserialized by the provider when Create is called.
	Config json.RawMessage `json:"config"`
}

// UnmarshalConfig performs a strict JSON unmarshal of the config to the desired struct.
func (r *CredentialRequest) UnmarshalConfig(target interface{}) error {
	if err := UnmarshalConfig(r.Config, target); err != nil {
		return fmt.Errorf("%s request: unmarshal: %s", r.Type, err)
	}
	return nil
}

// hasValidCredentials returns true if there are already valid credentials
// for the request. This is determined by the last resource state.
func (r *CredentialRequest) hasValidCredentials(resource *Resource, rotationWindow time.Duration) bool {

	if resource.Deposed {
		return false
	}
	if r.Name != resource.ID {
		return false
	}
	if !isEqualConfig(r.Config, resource.Config) {
		return false
	}

	rotation := rotationWindow
	if r.RotationWindow != 0 {
		rotation = time.Duration(r.RotationWindow) * time.Second
	}

	if resource.Expiration.Add(-rotation).Before(time.Now()) {
		return false
	}

	return true
}

// CredentialType ...
type CredentialType string

// Enumeration of known credential types.
const (
	Randomized             CredentialType = "random"
	AWSSTS                 CredentialType = "aws:sts"
	GithubDeployKey        CredentialType = "github:deploy-key"
	GithubAccessToken      CredentialType = "github:access-token"
	ArtifactoryAccessToken CredentialType = "artifactory:access-token"
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
	case ArtifactoryAccessToken:
		return Artifactory
	}
	return ProviderType(c)
}

// Enumeration of known provider types.
const (
	Random      ProviderType = "random"
	AWS         ProviderType = "aws"
	Github      ProviderType = "github"
	Artifactory ProviderType = "artifactory"
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
	Create(request *CredentialRequest) ([]*Credential, *Metadata, error)

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
	GithubSecrets  StoreType = "github"
)

// StoreType ...
type StoreType string

// SecretStore is implemented by store backends for secrets.
type SecretStore interface {
	// Type returns the store type.
	Type() StoreType

	// Write a sidecred.Credential to the secret store.
	Write(namespace string, secret *Credential, config json.RawMessage) (string, error)

	// Read the specified secret by reference.
	Read(path string, config json.RawMessage) (string, bool, error)

	// Delete the specified secret. Should not return an error
	// if the secret does not exist or has already been deleted.
	Delete(path string, config json.RawMessage) error
}

// BuildSecretTemplate is a convenience function for building secret templates.
func BuildSecretTemplate(secretTemplate, namespace, name string) (string, error) {
	t, err := template.New("path").Option("missingkey=error").Parse(secretTemplate)
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
func New(providers []Provider, stores []SecretStore, rotationWindow time.Duration, logger *zap.Logger) (*Sidecred, error) {
	s := &Sidecred{
		providers:      make(map[ProviderType]Provider, len(providers)),
		stores:         make(map[StoreType]SecretStore, len(stores)),
		rotationWindow: rotationWindow,
		logger:         logger,
	}
	for _, p := range providers {
		s.providers[p.Type()] = p
	}
	for _, t := range stores {
		s.stores[t.Type()] = t
	}
	return s, nil
}

// Sidecred is the underlying datastructure for the service.
type Sidecred struct {
	providers      map[ProviderType]Provider
	stores         map[StoreType]SecretStore
	rotationWindow time.Duration
	logger         *zap.Logger
}

// Process a single sidecred.Request.
func (s *Sidecred) Process(config *Config, state *State) error {
	log := s.logger.With(zap.String("namespace", config.Namespace))
	log.Info("starting sidecred", zap.Int("requests", len(config.Requests)))

	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid config: %s", err)
	}

RequestLoop:
	for _, request := range config.Requests {
		var (
			store       SecretStore
			storeConfig *StoreConfig
		)
		for _, sc := range config.Stores {
			if sc.alias() == request.Store {
				storeConfig = sc
			}
		}
		if _, enabled := s.stores[storeConfig.Type]; !enabled {
			log.Warn("store type is not enabled", zap.String("storeType", request.Store))
			continue RequestLoop
		}
		store = s.stores[storeConfig.Type]
		if store == nil {
			log.Warn("store does not exist", zap.String("store", request.Store))
			continue RequestLoop
		}

	CredentialLoop:
		for _, r := range request.CredentialRequests() {
			log := log.With(zap.String("type", string(r.Type)), zap.String("store", request.Store))
			if r.Name == "" {
				log.Warn("missing name in request")
				continue CredentialLoop
			}
			p, ok := s.providers[r.Type.Provider()]
			if !ok {
				log.Warn("provider not configured")
				continue CredentialLoop
			}
			log.Info("processing request", zap.String("name", r.Name))

			for _, resource := range state.GetResourcesByID(r.Type, r.Name, storeConfig.alias()) {
				if r.hasValidCredentials(resource, s.rotationWindow) {
					log.Info("found existing credentials", zap.String("name", r.Name))
					continue CredentialLoop
				}
			}

			creds, metadata, err := p.Create(r)
			if err != nil {
				log.Error("failed to provide credentials", zap.Error(err))
				continue CredentialLoop
			}
			if len(creds) == 0 {
				log.Error("no credentials returned by provider")
				continue CredentialLoop
			}
			state.AddResource(newResource(r, storeConfig.alias(), creds[0].Expiration, metadata))
			log.Info("created new credentials", zap.Int("count", len(creds)))

			for _, c := range creds {
				path, err := store.Write(config.Namespace, c, storeConfig.Config)
				if err != nil {
					log.Error("store credential", zap.String("name", c.Name), zap.Error(err))
					continue
				}
				state.AddSecret(storeConfig, newSecret(r.Name, path, c.Expiration))
				log.Debug("stored credential", zap.String("path", path))
			}
			log.Info("done processing")
		}
	}

	for _, ps := range state.Providers {
		// Reverse loop to handle index changes due to deleting items in the
		// underlying array: https://stackoverflow.com/a/29006008
		for i := len(ps.Resources) - 1; i >= 0; i-- {
			resource := ps.Resources[i]
			if resource.InUse && !resource.Deposed {
				continue
			}
			provider, ok := s.providers[ps.Type]
			if !ok {
				log.Debug("missing provider for expired resource", zap.String("type", string(ps.Type)))
				continue
			}
			log := s.logger.With(
				zap.String("type", string(ps.Type)),
				zap.String("id", resource.ID),
			)
			log.Info("destroying expired resource")
			if err := provider.Destroy(resource); err != nil {
				log.Error("destroy resource", zap.Error(err))
			}
			state.RemoveResource(resource)
		}
	}

	for _, ss := range state.Stores {
		log := log.With(zap.String("storeType", string(ss.StoreConfig.Type)))
		orphans := state.ListOrphanedSecrets(ss.StoreConfig)
		for i := len(orphans) - 1; i >= 0; i-- {
			secret := orphans[i]
			store, ok := s.stores[ss.StoreConfig.Type]
			if !ok {
				log.Debug("missing store for expired secret")
				continue
			}
			log.Info("deleting orphaned secret", zap.String("path", secret.Path))
			if err := store.Delete(secret.Path, ss.StoreConfig.Config); err != nil {
				log.Error("delete secret", zap.String("path", secret.Path), zap.Error(err))
			}
			state.RemoveSecret(ss.StoreConfig, secret)
		}
	}
	return nil
}
