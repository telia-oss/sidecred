// Package github implements a sidecred.Provider for Github access tokens and deploy keys. It also implements
// a client for Github Apps, which is used to create the supported credentials.
package github

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strconv"
	"time"

	"github.com/telia-oss/sidecred"

	"github.com/google/go-github/v29/github"
	"github.com/telia-oss/githubapp"
	"golang.org/x/crypto/ssh"
)

var (
	_ sidecred.Validatable = &DeployKeyRequestConfig{}
	_ sidecred.Validatable = &AccessTokenRequestConfig{}
)

// DeployKeyRequestConfig ...
type DeployKeyRequestConfig struct {
	Owner      string `json:"owner"`
	Repository string `json:"repository"`
	Title      string `json:"title"`
	ReadOnly   bool   `json:"read_only"`
}

// Validate implements sidecred.Validatable.
func (c *DeployKeyRequestConfig) Validate() error {
	if c.Owner == "" {
		return fmt.Errorf("%q must be defined", "owner")
	}
	if c.Repository == "" {
		return fmt.Errorf("%q must be defined", "repository")
	}
	if c.Repository == "" {
		return fmt.Errorf("%q must be defined", "title")
	}
	return nil
}

// AccessTokenRequestConfig ...
type AccessTokenRequestConfig struct {
	Owner        string                 `json:"owner"`
	Repositories []string               `json:"repositories,omitempty"`
	Permissions  *githubapp.Permissions `json:"permissions,omitempty"`
}

// Validate implements sidecred.Validatable.
func (c *AccessTokenRequestConfig) Validate() error {
	if c.Owner == "" {
		return fmt.Errorf("%q must be defined", "owner")
	}
	return nil
}

// New returns a new sidecred.Provider for Github credentials.
func New(app App, opts Options) sidecred.Provider {
	if opts.DeployKeyRotationInterval == 0 {
		opts.DeployKeyRotationInterval = 24 * 7 * time.Hour
	}
	if opts.ReposClientFactory == nil {
		opts.ReposClientFactory = func(token string) RepositoriesAPI {
			return githubapp.NewInstallationClient(token).V3.Repositories
		}
	}
	return &provider{
		app:  app,
		opts: opts,
		defaultTokenPermissions: &githubapp.Permissions{
			Metadata:     github.String("read"),
			Contents:     github.String("read"),
			PullRequests: github.String("write"),
			Statuses:     github.String("write"),
		},
	}
}

// Options for the provider.
type Options struct {
	// DeployKeyRotationInterval sets the interval at which deploy keys should be rotated.
	DeployKeyRotationInterval time.Duration

	// ReposClientFactory sets the function used to create new installation clients, and can be used to return test fakes.
	ReposClientFactory func(token string) RepositoriesAPI
}

// Implements sidecred.Provider for Github Credentials.
type provider struct {
	app                     App
	opts                    Options
	defaultTokenPermissions *githubapp.Permissions
}

// Type implements sidecred.Provider.
func (p *provider) Type() sidecred.ProviderType {
	return sidecred.Github
}

// Create implements sidecred.Provider.
func (p *provider) Create(request *sidecred.CredentialRequest) ([]*sidecred.Credential, *sidecred.Metadata, error) {
	switch request.Type {
	case sidecred.GithubDeployKey:
		return p.createDeployKey(request)
	case sidecred.GithubAccessToken:
		return p.createAccessToken(request)
	}
	return nil, nil, fmt.Errorf("invalid request: %s", request.Type)
}

func (p *provider) createAccessToken(request *sidecred.CredentialRequest) ([]*sidecred.Credential, *sidecred.Metadata, error) {
	var c AccessTokenRequestConfig
	if err := request.UnmarshalConfig(&c); err != nil {
		return nil, nil, err
	}
	permissions := p.defaultTokenPermissions
	if c.Permissions != nil {
		permissions = c.Permissions
	}
	token, err := p.app.CreateInstallationToken(c.Owner, c.Repositories, permissions)
	if err != nil {
		return nil, nil, fmt.Errorf("create access token: %s", err)
	}
	return []*sidecred.Credential{{
		Name:        c.Owner + "-access-token",
		Value:       token.GetToken(),
		Description: "Github access token managed by sidecred.",
		Expiration:  token.GetExpiresAt().UTC(),
	}}, nil, nil
}

func (p *provider) createDeployKey(request *sidecred.CredentialRequest) ([]*sidecred.Credential, *sidecred.Metadata, error) {
	var c DeployKeyRequestConfig
	if err := request.UnmarshalConfig(&c); err != nil {
		return nil, nil, err
	}
	token, err := p.app.CreateInstallationToken(c.Owner, []string{c.Repository}, &githubapp.Permissions{
		Administration: github.String("write"), // Used to add deploy keys to repositories: https://developer.github.com/v3/apps/permissions/#permission-on-administration
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create administrator access token: %s", err)
	}

	privateKey, publicKey, err := p.generateKeyPair()
	if err != nil {
		return nil, nil, fmt.Errorf("generate key pair: %s", err)
	}

	key, _, err := p.opts.ReposClientFactory(token.GetToken()).CreateKey(context.TODO(), c.Owner, c.Repository, &github.Key{
		ID:       nil,
		Key:      github.String(publicKey),
		URL:      nil,
		Title:    github.String(c.Title),
		ReadOnly: github.Bool(c.ReadOnly),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create deploy key: %s", err)
	}

	metadata := &sidecred.Metadata{"key_id": strconv.Itoa(int(key.GetID()))}
	return []*sidecred.Credential{{
		Name:        c.Repository + "-deploy-key",
		Value:       privateKey,
		Description: "Github deploy key managed by sidecred.",
		Expiration:  key.GetCreatedAt().Add(p.opts.DeployKeyRotationInterval).UTC(),
	}}, metadata, nil
}

func (p *provider) generateKeyPair() (string, string, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}

	privateKey := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	pub, err := ssh.NewPublicKey(&key.PublicKey)
	if err != nil {
		return "", "", err
	}
	publicKey := ssh.MarshalAuthorizedKey(pub)
	return string(privateKey), string(publicKey), nil
}

// Destroy implements sidecred.Provider.
func (p *provider) Destroy(resource *sidecred.Resource) error {
	var c DeployKeyRequestConfig
	if err := json.Unmarshal(resource.Config, &c); err != nil {
		return fmt.Errorf("unmarshal resource config: %s", err)
	}
	if resource.Metadata == nil {
		return nil
	}
	s := (*resource.Metadata)["key_id"]
	if s == "" {
		return nil
	}
	keyID, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("failed to convert key id (%s) to int: %s", s, err)
	}
	token, err := p.app.CreateInstallationToken(c.Owner, []string{c.Repository}, &githubapp.Permissions{
		Administration: github.String("write"), // Used to add deploy keys to repositories: https://developer.github.com/v3/apps/permissions/#permission-on-administration
	})
	if err != nil {
		return fmt.Errorf("create administrator access token: %s", err)
	}
	resp, err := p.opts.ReposClientFactory(token.GetToken()).DeleteKey(context.TODO(), c.Owner, c.Repository, keyID)
	if err != nil {
		// Ignore error if status code is 404 (key not found)
		if resp == nil || resp.StatusCode != 404 {
			return fmt.Errorf("delete deploy key: %s", err)
		}
	}
	return nil
}

// App is the interface that needs to be satisfied by the Github App implementation.
//
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . App
type App interface {
	CreateInstallationToken(owner string, repositories []string, permissions *githubapp.Permissions) (*githubapp.Token, error)
}

// RepositoriesAPI wraps the Github repositories API.
//
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . RepositoriesAPI
type RepositoriesAPI interface {
	ListKeys(ctx context.Context, owner string, repo string, opt *github.ListOptions) ([]*github.Key, *github.Response, error)
	CreateKey(ctx context.Context, owner string, repo string, key *github.Key) (*github.Key, *github.Response, error)
	DeleteKey(ctx context.Context, owner string, repo string, id int64) (*github.Response, error)
}
