package github

import (
	"github.com/telia-oss/githubapp"

	"github.com/telia-oss/sidecred"
)

func NewActionsStore(app App, options ...Option) sidecred.SecretStore {
	options = append(options, forStoreType(sidecred.GithubSecrets), WithActionsClientFactory(func(token string) ActionsAPI {
		return githubapp.NewInstallationClient(token).V3.Actions
	}))

	return NewStore(app, options...)
}
