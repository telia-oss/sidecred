package githubrotator

import (
	"context"
	"fmt"
	"strconv"

	"github.com/google/go-github/v45/github"
	"github.com/telia-oss/githubapp"
)

//counterfeiter:generate -o fakes/app.go . App
type App interface {
	CreateInstallationToken(owner string, repositories []string, permissions *githubapp.Permissions) (*githubapp.Token, error)
}

//counterfeiter:generate -o fakes/appfactory.go . AppFactory
type AppFactory interface {
	Create(integrationID, privateKey string) (App, error)
}

//counterfeiter:generate -o fakes/ratelimits.go . RateLimits
type RateLimits interface {
	GetTokenRateLimits(ctx context.Context, token string) (*github.RateLimits, *github.Response, error)
}

// ---

type defaultAppFactory struct{}

func (defaultAppFactory) Create(integrationID, privateKey string) (App, error) {

	intIntegrationID, err := strconv.ParseInt(integrationID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("")
	}

	client, err := githubapp.NewClient(intIntegrationID, []byte(privateKey))
	if err != nil {
		return nil, fmt.Errorf("")
	}

	return githubapp.New(client), nil
}

type defaultRateLimitClient struct{}

func (defaultRateLimitClient) GetTokenRateLimits(ctx context.Context, token string) (*github.RateLimits, *github.Response, error) {
	return githubapp.NewInstallationClient(token).V3.RateLimits(ctx)
}
