// Package artifactory implements a sidecred.Provider for Artifactory access token credentials.
package artifactory

import (
	"fmt"
	"os"
	"time"

	"github.com/telia-oss/sidecred"

	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/auth"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// RequestConfig ...
type RequestConfig struct {
	User     string `json:"user"`
	Group    string `json:"group"`
	Duration int    `json:"duration"`
}

// NewClient returns a new client for ArtifactoryAPI.
func NewClient(hostname string, username string, password string, accessToken string, apiKey string) (ArtifactoryAPI, error) {
	rtDetails := auth.NewArtifactoryDetails()
	rtDetails.SetUrl(hostname)
	rtDetails.SetUser(username)
	rtDetails.SetPassword(password)
	rtDetails.SetAccessToken(accessToken)
	rtDetails.SetApiKey(apiKey)

	serviceConfig, err := artifactory.NewConfigBuilder().
		SetArtDetails(rtDetails).
		Build()
	if err != nil {
		return nil, err
	}

	log.SetLogger(log.NewLogger(log.DEBUG, os.Stdout))

	return artifactory.New(&rtDetails, serviceConfig)
}

// New returns a new sidecred.Provider for Artifactory Credentials.
func New(client ArtifactoryAPI, options ...option) sidecred.Provider {
	p := &provider{
		client:          client,
		sessionDuration: time.Duration(1 * time.Hour),
	}
	for _, optionFunc := range options {
		optionFunc(p)
	}
	return p
}

type option func(*provider)

// WithSessionDuration overrides the default session duration.
func WithSessionDuration(duration time.Duration) option {
	return func(p *provider) {
		p.sessionDuration = duration
	}
}

type provider struct {
	client          ArtifactoryAPI
	sessionDuration time.Duration
}

// Type implements sidecred.Provider.
func (p *provider) Type() sidecred.ProviderType {
	return sidecred.Artifactory
}

// Provide implements sidecred.Provider.
func (p *provider) Create(request *sidecred.Request) ([]*sidecred.Credential, *sidecred.Metadata, error) {
	var c RequestConfig
	if err := request.UnmarshalConfig(&c); err != nil {
		return nil, nil, err
	}
	duration := int(p.sessionDuration.Seconds())
	if c.Duration != 0 {
		duration = c.Duration
	}

	params := services.CreateTokenParams{
		Scope:       fmt.Sprintf("api:* member-of-groups:%s", c.Group),
		Username:    c.User,
		ExpiresIn:   duration,
		Refreshable: false,
		Audience:    "",
	}

	expiry := time.Now().UTC().Add(time.Duration(duration) * time.Second)

	output, err := p.client.CreateToken(params)
	if err != nil {
		return nil, nil, fmt.Errorf("create token: %s", err)
	}

	return []*sidecred.Credential{
		{
			Name:        request.Name + "-artifactory-user",
			Value:       c.User,
			Expiration:  expiry,
			Description: "Artifactory credentials managed by sidecred.",
		},
		{
			Name:        request.Name + "-artifactory-token",
			Value:       output.AccessToken,
			Expiration:  expiry,
			Description: "Artifactory credentials managed by sidecred.",
		},
	}, nil, nil
}

// Destroy implements sidecred.Provider.
func (p *provider) Destroy(resource *sidecred.Resource) error {
	return nil
}

// ArtifactoryAPI wraps the Artifactory access token API.
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . ArtifactoryAPI
type ArtifactoryAPI interface {
	CreateToken(services.CreateTokenParams) (services.CreateTokenResponseData, error)
}
