// Package github implements a sidecred.SecretStore on top of Github secrets.
package github

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/telia-oss/githubapp"
	"github.com/telia-oss/sidecred"

	"github.com/google/go-github/v41/github"
	"golang.org/x/crypto/nacl/box"
)

// New creates a new sidecred.SecretStore using Github repository secrets.
func New(app App, options ...option) sidecred.SecretStore {
	s := &store{
		app:            app,
		keys:           make(map[string]*github.PublicKey),
		secretTemplate: "{{ .Namespace }}_{{ .Name }}",
		actionsClientFactory: func(token string) ActionsAPI {
			return githubapp.NewInstallationClient(token).V3.Actions
		},
	}
	for _, optionFunc := range options {
		optionFunc(s)
	}
	return s
}

type option func(*store)

// WithSecretTemplate sets the secret name template when instantiating a new store.
func WithSecretTemplate(t string) option {
	return func(s *store) {
		s.secretTemplate = t
	}
}

// WithActionsClientFactory sets the function used to create new installation clients, and can be used to return test fakes.
func WithActionsClientFactory(f func(token string) ActionsAPI) option {
	return func(s *store) {
		s.actionsClientFactory = f
	}
}

type store struct {
	app                  App
	keys                 map[string]*github.PublicKey
	actionsClientFactory func(token string) ActionsAPI
	secretTemplate       string
}

// config that can be passed to the Configure method of this store.
type config struct {
	SecretTemplate string `json:"secret_template"`
	RepositorySlug string `json:"repository"`

	// Fields populated when the config is parsed
	owner      string
	repository string
}

// Type implements sidecred.SecretStore.
func (s *store) Type() sidecred.StoreType {
	return sidecred.GithubSecrets
}

// Write implements sidecred.SecretStore.
func (s *store) Write(namespace string, secret *sidecred.Credential, config json.RawMessage) (string, error) {
	c, err := s.parseConfig(config)
	if err != nil {
		return "", fmt.Errorf("parse config: %w", err)
	}
	path, err := sidecred.BuildSecretTemplate(c.SecretTemplate, namespace, secret.Name)
	if err != nil {
		return "", fmt.Errorf("build secret path: %w", err)
	}
	// TODO: Scope token to "secrets" once go-github supports it:
	// https://developer.github.com/v3/apps/permissions/#permission-on-secrets
	//
	// It is not supported as of v32 of go-github:
	// https://github.com/google/go-github/blob/v32.1.0/github/apps.go#L60
	token, err := s.app.CreateInstallationToken(c.owner, []string{c.repository}, nil)
	if err != nil {
		return "", fmt.Errorf("create secrets access token: %w", err)
	}

	if _, found := s.keys[c.RepositorySlug]; !found {
		key, _, err := s.actionsClientFactory(token.GetToken()).GetRepoPublicKey(context.TODO(), c.owner, c.repository)
		if err != nil {
			return "", fmt.Errorf("get public key: %w", err)
		}
		s.keys[c.RepositorySlug] = key
	}
	publicKey := s.keys[c.RepositorySlug]

	encryptedSecret, err := s.encryptSecretValue(secret, publicKey)
	if err != nil {
		return "", fmt.Errorf("encrypt secret: %w", err)
	}

	path, err = s.sanitizeSecretPath(path)
	if err != nil {
		return "", fmt.Errorf("sanitize path: %w", err)
	}

	_, err = s.actionsClientFactory(token.GetToken()).CreateOrUpdateRepoSecret(context.TODO(), c.owner, c.repository, &github.EncryptedSecret{
		Name:           path,
		KeyID:          publicKey.GetKeyID(),
		EncryptedValue: encryptedSecret,
	})
	if err != nil {
		return "", fmt.Errorf("Actions.CreateOrUpdateRepoSecret returned error: %w", err)
	}

	return path, nil
}

// Read implements sidecred.SecretStore.
//
// TODO: Remove Read from SecretStore interface and return structs from New etc. Then rewrite Read for tests only.
func (s *store) Read(path string, config json.RawMessage) (string, bool, error) {
	c, err := s.parseConfig(config)
	if err != nil {
		return "", false, fmt.Errorf("parse config: %w", err)
	}
	token, err := s.app.CreateInstallationToken(c.owner, []string{c.repository}, nil)
	if err != nil {
		return "", false, fmt.Errorf("create secrets access token: %w", err)
	}
	secret, _, err := s.actionsClientFactory(token.GetToken()).GetRepoSecret(context.TODO(), c.owner, c.repository, path)
	if err != nil {
		return "", false, fmt.Errorf("get secret: %w", err)
	}
	return secret.Name, true, nil
}

// Delete implements sidecred.SecretStore.
func (s *store) Delete(path string, config json.RawMessage) error {
	c, err := s.parseConfig(config)
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	token, err := s.app.CreateInstallationToken(c.owner, []string{c.repository}, nil)
	if err != nil {
		return fmt.Errorf("create secrets access token: %w", err)
	}
	resp, err := s.actionsClientFactory(token.GetToken()).DeleteRepoSecret(context.TODO(), c.owner, c.repository, path)
	if err != nil {
		// Assume that the secret no longer exists if a 404 error is encountered
		if resp == nil || resp.StatusCode != 404 {
			return fmt.Errorf("delete secret: %w", err)
		}
	}
	return nil
}

// parseConfig parses and validates the config.
func (s *store) parseConfig(raw json.RawMessage) (*config, error) {
	c := &config{}
	if err := sidecred.UnmarshalConfig(raw, &c); err != nil {
		return nil, err
	}
	if c.RepositorySlug == "" {
		return nil, fmt.Errorf("%q must be defined", "repository")
	}
	parts := strings.Split(c.RepositorySlug, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repository slug: %q", c.RepositorySlug)
	}
	c.owner, c.repository = parts[0], parts[1]
	if c.SecretTemplate == "" {
		c.SecretTemplate = s.secretTemplate
	}
	return c, nil
}

// encryptSecretValue encrypts the secret with a public key from Github.
func (s *store) encryptSecretValue(secret *sidecred.Credential, publicKey *github.PublicKey) (string, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(publicKey.GetKey())
	if err != nil {
		return "", fmt.Errorf("base64.StdEncoding.DecodeString was unable to decode public key: %w", err)
	}

	var key [32]byte
	copy(key[:], keyBytes)

	var out []byte
	encrypted, err := box.SealAnonymous(out, []byte(secret.Value), &key, nil)
	if err != nil {
		return "", fmt.Errorf("unable to encrypt with key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// sanitizeSecretPath replaces all illegal characters in the path with "_" (underscore) and makes the name uppercase. See link for legal names:
// https://docs.github.com/en/actions/configuring-and-managing-workflows/creating-and-storing-encrypted-secrets#naming-your-secrets
func (s *store) sanitizeSecretPath(path string) (string, error) {
	re, err := regexp.Compile("[^a-zA-Z0-9]+")
	if err != nil {
		return "", err
	}
	sp := re.ReplaceAllString(path, "_")
	return strings.ToUpper(sp), nil
}

// App is the interface that needs to be satisfied by the Github App implementation.
//
//counterfeiter:generate . App
type App interface {
	CreateInstallationToken(owner string, repositories []string, permissions *githubapp.Permissions) (*githubapp.Token, error)
}

// ActionsAPI wraps the Github actions API.
//
//counterfeiter:generate . ActionsAPI
type ActionsAPI interface {
	GetRepoPublicKey(ctx context.Context, owner, repo string) (*github.PublicKey, *github.Response, error)
	CreateOrUpdateRepoSecret(ctx context.Context, owner, repo string, eSecret *github.EncryptedSecret) (*github.Response, error)
	GetRepoSecret(ctx context.Context, owner, repo, name string) (*github.Secret, *github.Response, error)
	DeleteRepoSecret(ctx context.Context, owner, repo, name string) (*github.Response, error)
}
