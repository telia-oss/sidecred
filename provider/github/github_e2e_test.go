//go:build e2e

package github_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/telia-oss/sidecred"
	provider "github.com/telia-oss/sidecred/provider/github"

	"github.com/stretchr/testify/require"
	"github.com/telia-oss/githubapp"
)

var (
	appIntegrationID   = os.Getenv("GITHUB_APP_INTEGRATION_ID")
	appPrivateKeyFile  = os.Getenv("GITHUB_APP_PRIVATE_KEY_FILE")
	targetOrganisation = os.Getenv("GITHUB_APP_TARGET_ORG")
	targetRepository   = os.Getenv("GITHUB_APP_TARGET_REPOSITORY")
)

// Running the e2e tests for Github require that the above variables have been set in the environment.
// The integration ID and private key for a Github app that is set up with the correct permissions
// and has been installed for the target organisation and repository.
func TestGithubProviderE2E(t *testing.T) {
	tests := []struct {
		description string
	}{
		{
			description: "Github provider works",
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			integrationID, err := strconv.ParseInt(appIntegrationID, 10, 64)
			require.NoError(t, err)

			privateKey, err := ioutil.ReadFile(appPrivateKeyFile)
			require.NoError(t, err)

			client, err := githubapp.NewClient(integrationID, privateKey)
			require.NoError(t, err)

			request := &sidecred.CredentialRequest{
				Type: sidecred.GithubDeployKey,
				Name: "itsdalmo-dotfiles-deploy-key",
				Config: []byte(
					fmt.Sprintf(`{"owner":"%s","repository":"%s","title":"sidecred-e2e-test","read_only":true}`, targetOrganisation, targetRepository),
				),
			}

			p := provider.New(githubapp.New(client))
			_, metadata, err := p.Create(request)
			require.NoError(t, err)

			resource := &sidecred.Resource{
				ID:         "itsdalmo-dotfiles-deploy-key",
				Expiration: time.Now(),
				Config:     request.Config,
				Metadata:   metadata,
			}
			err = p.Destroy(resource)
			require.NoError(t, err)
		})
	}
}
