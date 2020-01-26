package github

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/v28/github"
)

// AppsAPI wraps the Github Apps API.
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . AppsAPI
type AppsAPI interface {
	ListInstallations(ctx context.Context, opt *github.ListOptions) ([]*github.Installation, *github.Response, error)
	CreateInstallationToken(ctx context.Context, id int64, opt *github.InstallationTokenOptions) (*github.InstallationToken, *github.Response, error)
}

// RepositoriesAPI wraps the Github repositories API.
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . RepositoriesAPI
type RepositoriesAPI interface {
	ListKeys(ctx context.Context, owner string, repo string, opt *github.ListOptions) ([]*github.Key, *github.Response, error)
	CreateKey(ctx context.Context, owner string, repo string, key *github.Key) (*github.Key, *github.Response, error)
	DeleteKey(ctx context.Context, owner string, repo string, id int64) (*github.Response, error)
}

// NewAppsClient returns a client for a Github App authenticated with a private key.
func NewAppsClient(integrationID int64, privateKey string) (AppsAPI, error) {
	transport, err := ghinstallation.NewAppsTransport(http.DefaultTransport, integrationID, []byte(privateKey))
	if err != nil {
		return nil, err
	}
	client := github.NewClient(&http.Client{
		Transport: transport,
	})
	return client.Apps, nil
}

func newApp(client AppsAPI) *app {
	return &app{
		client:         client,
		installations:  make(map[string]int64),
		updateInterval: 10 * time.Minute,
	}
}

type app struct {
	client         AppsAPI
	installations  map[string]int64
	updatedAt      time.Time
	updateInterval time.Duration
}

func (a *app) refreshInstallations() error {
	if nextUpdate := a.updatedAt.Add(a.updateInterval); nextUpdate.After(time.Now()) {
		return nil
	}

	// TODO: Paginate results.
	installations, _, err := a.client.ListInstallations(context.TODO(), &github.ListOptions{})
	if err != nil {
		return err
	}

	for _, i := range installations {
		owner := i.GetAccount().GetLogin()
		if owner == "" {
			return fmt.Errorf("missing owner for installation: %d", i.GetID())
		}
		a.installations[strings.ToLower(owner)] = i.GetID()
	}

	return nil
}

func (a *app) createInstallationToken(owner string, permissions *github.InstallationPermissions) (string, time.Time, error) {
	if err := a.refreshInstallations(); err != nil {
		return "", time.Time{}, fmt.Errorf("refresh installations: %s", err)
	}

	id, ok := a.installations[strings.ToLower(owner)]
	if !ok {
		return "", time.Time{}, fmt.Errorf("missing installation for user/org: '%s'", owner)
	}

	tokenOptions := &github.InstallationTokenOptions{
		Permissions: permissions,
	}

	installationToken, _, err := a.client.CreateInstallationToken(context.TODO(), id, tokenOptions)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("create token: %s", err)
	}
	return installationToken.GetToken(), installationToken.GetExpiresAt().UTC(), nil
}
