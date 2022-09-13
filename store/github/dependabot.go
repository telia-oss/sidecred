package github

import (
	"github.com/telia-oss/githubapp"

	"github.com/telia-oss/sidecred"
)

func NewDependabotStore(app App, options ...Option) sidecred.SecretStore {
	options = append(options, forStoreType(sidecred.GithubDependabotSecrets), WithActionsClientFactory(func(token string) ActionsAPI {
		return githubapp.NewInstallationClient(token).V3.Dependabot
	}))

	return NewStore(app, options...)
}
