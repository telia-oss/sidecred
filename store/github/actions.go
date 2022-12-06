package github

import (
	"github.com/telia-oss/githubapp"
	"go.uber.org/zap"

	"github.com/telia-oss/sidecred"
)

func NewActionsStore(app App, logger *zap.Logger, options ...Option) sidecred.SecretStore {
	options = append(options, forStoreType(sidecred.GithubSecrets), WithActionsClientFactory(func(token string) ActionsAPI {
		return githubapp.NewInstallationClient(token).V3.Actions
	}))

	return NewStore(app, logger, options...)
}
