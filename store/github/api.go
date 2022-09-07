package github

import (
	"context"

	"github.com/google/go-github/v45/github"
)

// ActionsAPI wraps the Github actions API.
//
//counterfeiter:generate . ActionsAPI
type ActionsAPI interface {
	GetRepoPublicKey(ctx context.Context, owner, repo string) (*github.PublicKey, *github.Response, error)
	CreateOrUpdateRepoSecret(
		ctx context.Context,
		owner, repo string,
		eSecret *github.EncryptedSecret,
	) (*github.Response, error)
	GetRepoSecret(ctx context.Context, owner, repo, name string) (*github.Secret, *github.Response, error)
	DeleteRepoSecret(ctx context.Context, owner, repo, name string) (*github.Response, error)
}
