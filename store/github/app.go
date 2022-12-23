package github

import (
	"context"

	"github.com/telia-oss/githubapp"
)

// App is the interface that needs to be satisfied by the Github App implementation.
//
//counterfeiter:generate . App
type App interface {
	CreateInstallationToken(ctx context.Context, owner string, repositories []string, permissions *githubapp.Permissions) (
		*githubapp.Token,
		error,
	)
}
