// Package github implements a sidecred.SecretStore on top of Github secrets.
package github

import (
	"context"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"

	"github.com/telia-oss/githubapp"
	"github.com/telia-oss/sidecred"

	"github.com/google/go-github/v29/github"
	"golang.org/x/crypto/nacl/box"
)

// New creates a new sidecred.SecretStore using Github repository secrets.
func New(app App, owner, repository string, options ...option) sidecred.SecretStore {
	s := &store{
		app:            app,
		owner:          owner,
		repository:     repository,
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

// WithSecretTemplate sets the secret name template when instanciating a new store.
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
	owner                string
	repository           string
	keys                 map[string]*github.PublicKey
	actionsClientFactory func(token string) ActionsAPI
	secretTemplate       string
}

// Type implements sidecred.SecretStore.
func (s *store) Type() sidecred.StoreType {
	return sidecred.GithubSecrets
}

// Write implements sidecred.SecretStore.
func (s *store) Write(namespace string, secret *sidecred.Credential) (string, error) {
	path, err := sidecred.BuildSecretPath(s.secretTemplate, namespace, secret.Name)
	if err != nil {
		return "", fmt.Errorf("build secret path: %s", err)
	}
	// TODO: Scope token to "secrets" once go-github supports it:
	// https://developer.github.com/v3/apps/permissions/#permission-on-secrets
	//
	// It is not supported as of v32 of go-github:
	// https://github.com/google/go-github/blob/v32.1.0/github/apps.go#L60
	token, err := s.app.CreateInstallationToken(s.owner, []string{s.repository}, nil)
	if err != nil {
		return "", fmt.Errorf("create secrets access token: %s", err)
	}

	repositorySlug := s.owner + "/" + s.repository
	if _, found := s.keys[repositorySlug]; !found {
		key, _, err := s.actionsClientFactory(token.GetToken()).GetPublicKey(context.TODO(), s.owner, s.repository)
		if err != nil {
			return "", fmt.Errorf("get public key: %s", err)
		}
		s.keys[repositorySlug] = key
	}
	publicKey, _ := s.keys[repositorySlug]

	encryptedSecret, err := s.encryptSecretValue(secret, publicKey)
	if err != nil {
		return "", fmt.Errorf("encrypt secret: %s", err)
	}

	path, err = s.sanitizeSecretPath(path)
	if err != nil {
		return "", fmt.Errorf("sanitize path: %s", err)
	}

	_, err = s.actionsClientFactory(token.GetToken()).CreateOrUpdateSecret(context.TODO(), s.owner, s.repository, &github.EncryptedSecret{
		Name:           path,
		KeyID:          publicKey.GetKeyID(),
		EncryptedValue: encryptedSecret,
	})
	if err != nil {
		return "", err
	}

	return path, nil
}

// Read implements sidecred.SecretStore.
//
// TODO: Remove Read from SecretStore interface and return structs from New etc. Then rewrite Read for tests only.
func (s *store) Read(path string) (string, bool, error) {
	token, err := s.app.CreateInstallationToken(s.owner, []string{s.repository}, nil)
	if err != nil {
		return "", false, fmt.Errorf("create secrets access token: %s", err)
	}
	secret, _, err := s.actionsClientFactory(token.GetToken()).GetSecret(context.TODO(), s.owner, s.repository, path)
	if err != nil {
		return "", false, fmt.Errorf("get secret: %s", err)
	}
	return secret.Name, true, nil
}

// Delete implements sidecred.SecretStore.
func (s *store) Delete(path string) error {
	token, err := s.app.CreateInstallationToken(s.owner, []string{s.repository}, nil)
	if err != nil {
		return fmt.Errorf("create secrets access token: %s", err)
	}
	resp, err := s.actionsClientFactory(token.GetToken()).DeleteSecret(context.TODO(), s.owner, s.repository, path)
	if err != nil {
		// Assume that the secret no longer exists if a 404 error is encountered
		if resp.StatusCode != 404 {
			return fmt.Errorf("delete secret: %s", err)
		}
	}
	return nil
}

func (s *store) encryptSecretValue(secret *sidecred.Credential, publicKey *github.PublicKey) (string, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(publicKey.GetKey())
	if err != nil {
		return "", err
	}

	var key [32]byte
	copy(key[:], keyBytes)

	var out []byte
	encrypted, err := box.SealAnonymous(out, []byte(secret.Value), &key, nil)
	if err != nil {
		return "", nil
	}
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// sanitizeSecretPath replaces all illegal characters in the path with "_" (underscore) and uppercases the name. See link for legal names:
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
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . App
type App interface {
	CreateInstallationToken(owner string, repositories []string, permissions *githubapp.Permissions) (*githubapp.Token, error)
}

// ActionsAPI wraps the Github actions API.
//
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . ActionsAPI
type ActionsAPI interface {
	GetPublicKey(ctx context.Context, owner, repo string) (*github.PublicKey, *github.Response, error)
	CreateOrUpdateSecret(ctx context.Context, owner, repo string, eSecret *github.EncryptedSecret) (*github.Response, error)
	GetSecret(ctx context.Context, owner, repo, name string) (*github.Secret, *github.Response, error)
	DeleteSecret(ctx context.Context, owner, repo, name string) (*github.Response, error)
}
