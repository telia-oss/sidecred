package github

import (
	"fmt"

	"github.com/google/go-github/v45/github"
	"github.com/telia-oss/githubapp"
)

// App is the interface that needs to be satisfied by the Github App implementation.
//
//counterfeiter:generate . App
type App interface {
	CreateInstallationToken(owner string, repositories []string, permissions *githubapp.Permissions) (
		*githubapp.Token,
		error,
	)
	RateLimits() (*github.RateLimits, *github.Response, error)
}

type MultiApp []App

func (apps MultiApp) CreateInstallationToken(owner string, repositories []string, permissions *githubapp.Permissions) (*githubapp.Token, error) {
	var err error
	for _, app := range apps {
		// adds call to RateLimits here and continues to the next app if remaining is 0
		rateLimits, _, err := app.RateLimits()
		switch {
		case err != nil:
			fmt.Printf("rate limits error: %s", err)
			continue
		case rateLimits == nil || rateLimits.Core == nil:
			fmt.Printf("rateLimits or rateLimits.Core is nil")
			continue
		case rateLimits.Core.Remaining == 0:
			fmt.Printf("rateLimits.Core.Remaining is 0")
			continue
		}
		token, err := app.CreateInstallationToken(owner, repositories, permissions)
		if err == nil {
			return token, nil
		}
	}

	return nil, fmt.Errorf("create secrets access token: %w", err)
}

func (apps MultiApp) RateLimits() (*github.RateLimits, *github.Response, error) {
	return nil, nil, nil
}
