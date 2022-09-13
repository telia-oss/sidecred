//go:build e2e

package github

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"testing"

	"github.com/telia-oss/sidecred"

	"github.com/stretchr/testify/assert"
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
func TestGithubStoreE2E(t *testing.T) {
	var (
		namespace   = "e2e"
		secretName  = "secret-name"
		secretValue = "secret-value"
	)

	tests := []struct {
		description  string
		pathTemplate string
		expectedPath string
	}{
		{
			description:  "github repository secrets works",
			pathTemplate: "sidecred_{{ .Namespace }}_{{ .Name }}",
			expectedPath: "SIDECRED_E2E_SECRET_NAME",
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

			store := newStore(
				githubapp.New(client),
				WithSecretTemplate(tc.pathTemplate),
			)
			assert.Equal(t, sidecred.GithubSecrets, store.Type())

			config := []byte(fmt.Sprintf(`{"repository":"%s/%s"}`, targetOrganisation, targetRepository))

			path, err := store.Write(namespace, &sidecred.Credential{
				Name:  secretName,
				Value: secretValue,
			}, config)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedPath, path)

			value, found, err := store.Read(path, config)
			assert.NoError(t, err, "read secret")
			assert.True(t, found, "found secret")
			assert.Equal(t, tc.expectedPath, value)

			if err := store.Delete(path, config); err != nil {
				t.Errorf("delete secret (%s): %s", path, err)
			}
		})
	}
}
