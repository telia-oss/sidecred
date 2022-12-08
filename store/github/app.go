package github

import (
	"fmt"

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
}

type MultiApp []App

func (apps MultiApp) CreateInstallationToken(owner string, repositories []string, permissions *githubapp.Permissions) (*githubapp.Token, error) {
	var err error
	for _, app := range apps {

		token, err := app.CreateInstallationToken(owner, repositories, permissions)
		if err == nil {
			fmt.Printf("created installation token, owner = %v\n", owner)
			return token, nil
		}
	}

	return nil, fmt.Errorf("create secrets access token: %w", err)
}
