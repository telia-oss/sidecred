// Package artifactory implements a sidecred.Provider for Artifactory access
// token credentials. See https://jfrog.com/artifactory/ for detailed
// information.
//
// The provider excercises the REST API to generate time limited access tokens
// (https://www.jfrog.com/confluence/display/JFROG/Access+Tokens). To access
// the API, the provider itself must be authenticated. The REST API generally
// supports the following authentication models (https://www.jfrog.com/confluence/display/JFROG/Artifactory+REST+API).
// Generally, this means we can authenticate with a dedicated username and
// password, where the password is one of the following:
//
//		API Key
//		Password
//		Access token
//
// The third is most desirable, as it means that we can allocate a revocable
// token under a specific username. Furthermore, that username can be a user
// allocated in Artifactory itself, as part of the call to issue a token. This
// avoids having to put an admin user's personal credentials into sidecred, or
// the API key, which have a higher blast radius if leaked.
package artifactory

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
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

var _ sidecred.Validatable = &RequestConfig{}

// RequestConfig ...
// The generated secrets will be `<name>-artifactory-user` and
// `<name>-artifactory-token`.
//
// The following shows an example resource configuration as YAML (note that the
// lambda version expects JSON):
//
//		- type: artifactory:access-token
//		  name: my-writer
//		  config:
//		    user: concourse-artifactory-user
//		    group: artifactory-writers-group
//		    duration: 30m
//
// For this specific example, the provider will create the secrets
// `my-writer-artifactory-user` and `my-writer-artifactory-token`. The value
// within the `my-writer-artifactory-user` secret will be
// `concourse-artifactory-user`.
// The secret `my-writer-artifactory-token` will contain the raw token.
type RequestConfig struct {
	// Username to allocate the credentials under.
	User string `json:"user"` //

	// Group to associate the credentials with.
	// The user will inherit the group permissions.
	Group string `json:"group"`

	// Request-specific override for credential duration.
	Duration *sidecred.Duration `json:"duration"`
}

// Validate implements sidecred.Validatable.
func (c *RequestConfig) Validate() error {
	if c.User == "" {
		return fmt.Errorf("%q must be defined", "user")
	}
	if c.Group == "" {
		return fmt.Errorf("%q must be defined", "group")
	}
	return nil
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
		sessionDuration: 1 * time.Hour,
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

// Create implements sidecred.Provider.
func (p *provider) Create(request *sidecred.CredentialRequest) ([]*sidecred.Credential, *sidecred.Metadata, error) {
	var c RequestConfig
	if err := request.UnmarshalConfig(&c); err != nil {
		return nil, nil, err
	}
	duration := int(p.sessionDuration.Seconds())
	if c.Duration != nil {
		duration = int(c.Duration.Seconds())
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
func (p *provider) Destroy(_ *sidecred.Resource) error {
	return nil
}

// ArtifactoryAPI wraps the Artifactory access token API.
//counterfeiter:generate . ArtifactoryAPI
type ArtifactoryAPI interface {
	CreateToken(services.CreateTokenParams) (services.CreateTokenResponseData, error)
}
