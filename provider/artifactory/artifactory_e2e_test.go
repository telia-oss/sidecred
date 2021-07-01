// +build e2e

package artifactory_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/telia-oss/sidecred"
	provider "github.com/telia-oss/sidecred/provider/artifactory"

	"github.com/stretchr/testify/require"
)

// See https://www.jfrog.com/confluence/display/RTF/Artifactory+REST+API for a full
// explanation of authentication options. Generally, you can authenticate with a
// username / password, username / access token, or username (optional) / API key.
// Thus, you do not need to specify all of these environment variables, but only
// the ones desired for the specific authentication method.
//
// If the token authentication method is used, it must be generated with the proper
// admin scope in order to mint tokens for users other than the caller.
var (
	artifactoryAPIHostname    = os.Getenv("ARTIFACTORY_API_HOSTNAME")
	artifactoryAPIUsername    = os.Getenv("ARTIFACTORY_API_USERNAME")
	artifactoryAPIPassword    = os.Getenv("ARTIFACTORY_API_PASSWORD")
	artifactoryAPIAccessToken = os.Getenv("ARTIFACTORY_API_ACCESS_TOKEN")
	artifactoryAPIKey         = os.Getenv("ARTIFACTORY_API_KEY")
	artifactoryUser           = os.Getenv("ARTIFACTORY_USER")
	artifactoryGroup          = os.Getenv("ARTIFACTORY_GROUP")
)

func TestArtifactoryProviderE2E(t *testing.T) {
	tests := []struct {
		description string
	}{
		{
			description: "artifactory provider works",
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			client, err := provider.NewClient(
				artifactoryAPIHostname,
				artifactoryAPIUsername,
				artifactoryAPIPassword,
				artifactoryAPIAccessToken,
				artifactoryAPIKey,
			)
			require.NoError(t, err)

			p := provider.New(client, provider.Options{
				SessionDuration: 15 * time.Minute,
			})

			_, _, err = p.Create(&sidecred.CredentialRequest{
				Type:   sidecred.ArtifactoryAccessToken,
				Name:   "request-name",
				Config: []byte(fmt.Sprintf(`{"user":"%s", "group":"%s"}`, artifactoryUser, artifactoryGroup)),
			})
			require.NoError(t, err)
		})
	}
}
