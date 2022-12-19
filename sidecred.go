package sidecred

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"text/template"
	"time"

	"go.uber.org/zap"
)

// Validatable allows sidecred to ensure the validity of the opaque config values used for processing a request.
type Validatable interface {
	Validate() error
}

// Config represents the user-defined configuration that should be passed when processing credentials using sidecred.
type Config interface {
	Validatable

	// Namespace (e.g. the name of a team, project or similar) to use when processing the credential requests.
	Namespace() string

	// Stores that can be targeted when mapping credentials.
	Stores() []*StoreConfig

	// Requests to map credentials to a secret store.
	Requests() []*CredentialsMap
}

// CredentialsMap represents a mapping between one or more credential request and a target secret store.
type CredentialsMap struct {
	// Store identifies the name or alias of the target secret store.
	Store string

	// Credentials that will be provisioned and written to the secret store.
	Credentials []*CredentialRequest
}

// CredentialRequest is the structure used to request credentials in Sidecred.
type CredentialRequest struct {
	// Type identifies the type of credential (and provider) for a request.
	Type CredentialType `json:"type"`

	// Name is an identifier that can be used for naming resources and
	// credentials created by a sidecred.Provider. The exact usage for
	// name is up to the individual provider.
	Name string `json:"name"`

	// Rotation is an override for the default rotation window
	// measured in seconds.
	// This will aid in cases where we want to be more granular
	// for possibly longer running authentications or processes.
	RotationWindow *Duration `json:"rotation_window"`

	// Config holds the provider configuration for the requested credential.
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
	if r.RotationWindow != nil {
		rotation = r.RotationWindow.Duration
	}
	if resource.Expiration.Add(-rotation).Before(time.Now()) {
		return false
	}
	return true
}

// UnmarshalConfig is a convenience method for performing a strict unmarshalling of a JSON config into a provided
// structure. If config is empty, no operation is performed by this function.
func UnmarshalConfig(config json.RawMessage, target interface{}) error {
	if len(config) == 0 {
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(config))
	d.DisallowUnknownFields()
	return d.Decode(target)
}

// isEqualConfig is a convenience function for unmarshalling the JSON config
// from the request and resource structures, and performing a logical deep
// equality check instead of a byte equality check. This avoids errors due to
// structural (but non-logical) changes due to (de)serialization.
func isEqualConfig(b1, b2 []byte) bool {
	var o1 interface{}
	var o2 interface{}

	// Allow the configurations to both be empty
	if len(b1) == 0 && len(b2) == 0 {
		return true
	}

	err := json.Unmarshal(b1, &o1)
	if err != nil {
		return false
	}

	err = json.Unmarshal(b2, &o2)
	if err != nil {
		return false
	}

	return reflect.DeepEqual(o1, o2)
}

// Duration implements JSON (un)marshal for time.Duration.
type Duration struct {
	time.Duration
}

// MarshalJSON implements json.Marshaler.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// UnmarshalJSON implements json.Unmarshaler.
func (d *Duration) UnmarshalJSON(data []byte) error {
	s, err := strconv.Unquote(string(data))
	if err != nil {
		return err
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("parse duration: %s", err)
	}
	d.Duration = v
	return nil
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
	Create(ctx context.Context, request *CredentialRequest) ([]*Credential, *Metadata, error)

	// Destroy the specified resource. This is scheduled if
	// a resource in the state has expired. For providers that
	// are not stateful this should be a no-op.
	Destroy(ctx context.Context, resource *Resource) error
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
	Inprocess               StoreType = "inprocess"
	SecretsManager          StoreType = "secretsmanager"
	SSM                     StoreType = "ssm"
	GithubSecrets           StoreType = "github"
	GithubDependabotSecrets StoreType = "github:dependabot"
)

// StoreType ...
type StoreType string

// StoreConfig is used to define the secret stores in the configuration for Sidecred.
type StoreConfig struct {
	Type   StoreType       `json:"type"`
	Name   string          `json:"name"`
	Config json.RawMessage `json:"config,omitempty"`
}

// Alias returns a name that can be used to identify configured store. defaults to the StoreType.
func (c *StoreConfig) Alias() string {
	if c.Name != "" {
		return c.Name
	}
	return string(c.Type)
}

// SecretStore is implemented by store backends for secrets.
type SecretStore interface {
	// Type returns the store type.
	Type() StoreType

	// Write a sidecred.Credential to the secret store.
	Write(ctx context.Context, namespace string, secret *Credential, config json.RawMessage) (string, error)

	// Read the specified secret by reference.
	Read(ctx context.Context, path string, config json.RawMessage) (string, bool, error)

	// Delete the specified secret. Should not return an error
	// if the secret does not exist or has already been deleted.
	Delete(ctx context.Context, path string, config json.RawMessage) error
}

// BuildSecretTemplate is a convenience function for building secret templates.
func BuildSecretTemplate(secretTemplate, namespace, name string) (string, error) {
	t, err := template.New("path").Option("missingkey=error").Parse(secretTemplate)
	if err != nil {
		return "", err
	}

	var p strings.Builder

	if err := t.Execute(&p, struct {
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

// Sidecred is the underlying structure for the service.
type Sidecred struct {
	providers      map[ProviderType]Provider
	stores         map[StoreType]SecretStore
	rotationWindow time.Duration
	logger         *zap.Logger
}

// Process a single sidecred.Request.
func (s *Sidecred) Process(ctx context.Context, config Config, state *State) error {
	log := s.logger.With(zap.String("namespace", config.Namespace()))
	log.Info("starting sidecred", zap.Int("requests", len(config.Requests())))

	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid config: %s", err)
	}

RequestLoop:
	for _, request := range config.Requests() {
		var storeConfig *StoreConfig
		for _, sc := range config.Stores() {
			if sc.Alias() == request.Store {
				storeConfig = sc
			}
		}
		if storeConfig == nil {
			log.Warn("could not find config for store", zap.String("store", request.Store))
			continue RequestLoop
		}
		store, enabled := s.stores[storeConfig.Type]
		if !enabled {
			log.Warn("store type is not enabled", zap.String("storeType", string(storeConfig.Type)))
			continue RequestLoop
		}

	CredentialLoop:
		for _, r := range request.Credentials {
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

			for _, resource := range state.GetResourcesByID(r.Type, r.Name, storeConfig.Alias()) {
				if r.hasValidCredentials(resource, s.rotationWindow) {
					log.Info("found existing credentials", zap.String("name", r.Name))
					continue CredentialLoop
				}
			}

			creds, metadata, err := p.Create(ctx, r)
			if err != nil {
				log.Error("failed to provide credentials", zap.Error(err))
				continue CredentialLoop
			}
			if len(creds) == 0 {
				log.Error("no credentials returned by provider")
				continue CredentialLoop
			}
			state.AddResource(newResource(r, storeConfig.Alias(), creds[0].Expiration, metadata))
			log.Info("created new credentials", zap.Int("count", len(creds)))

			for _, c := range creds {
				log.Debug("start creds for-loop")
				path, err := store.Write(ctx, config.Namespace(), c, storeConfig.Config)
				if err != nil {
					log.Error("store credential", zap.String("name", c.Name), zap.Error(err))
					continue
				}
				log.Debug("wrote to store", zap.String("name", c.Name))
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
			if err := provider.Destroy(ctx, resource); err != nil {
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
			if err := store.Delete(ctx, secret.Path, ss.StoreConfig.Config); err != nil {
				log.Error("delete secret", zap.String("path", secret.Path), zap.Error(err))
			}
			state.RemoveSecret(ss.StoreConfig, secret)
		}
	}
	return nil
}
