package artifactory_test

import (
	"testing"
	"time"

	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/telia-oss/sidecred"
	provider "github.com/telia-oss/sidecred/provider/artifactory"
	"github.com/telia-oss/sidecred/provider/artifactory/artifactoryfakes"
)

func TestArtifactoryProvider(t *testing.T) {
	expectedCredentials := []*sidecred.Credential{
		{
			Name:        "request-name-artifactory-user",
			Value:       "some-user",
			Description: "Artifactory credentials managed by sidecred.",
		},
		{
			Name:        "request-name-artifactory-token",
			Value:       "access-token",
			Description: "Artifactory credentials managed by sidecred.",
		},
	}
	tests := []struct {
		description             string
		sessionDuration         time.Duration
		expectedSessionDuration int64
		request                 *sidecred.CredentialRequest
		artifactoryAPIOutput    *services.CreateTokenResponseData
	}{
		{
			description:             "artifactory provider works",
			sessionDuration:         30 * time.Minute,
			expectedSessionDuration: 1800,
			request: &sidecred.CredentialRequest{
				Type:   sidecred.ArtifactoryAccessToken,
				Name:   "request-name",
				Config: []byte(`{"user": "some-user", "group": "some-artifactory-group"}`),
			},
		},
		{
			description:             "request duration overrides default",
			sessionDuration:         30 * time.Minute,
			expectedSessionDuration: 60,
			request: &sidecred.CredentialRequest{
				Type:   sidecred.AWSSTS,
				Name:   "request-name",
				Config: []byte(`{"user": "some-user", "group": "some-artifactory-group", "duration": 60}`),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			fakeArtifactoryAPI := &artifactoryfakes.FakeArtifactoryAPI{}
			fakeArtifactoryAPI.CreateTokenReturns(services.CreateTokenResponseData{
				ExpiresIn:   3600,
				AccessToken: "access-token",
			}, nil)

			p := provider.New(
				fakeArtifactoryAPI,
				provider.WithSessionDuration(tc.sessionDuration),
			)

			creds, metadata, err := p.Create(tc.request)
			require.NoError(t, err)
			require.Equal(t, 1, fakeArtifactoryAPI.CreateTokenCallCount())
			require.Len(t, creds, len(expectedCredentials))
			assert.Nil(t, metadata)

			for i, e := range expectedCredentials {
				assert.Equal(t, e.Name, creds[i].Name)
				assert.Equal(t, e.Value, creds[i].Value)
				assert.Equal(t, e.Description, creds[i].Description)
			}

			input := fakeArtifactoryAPI.CreateTokenArgsForCall(0)
			assert.Equal(t, tc.expectedSessionDuration, int64(input.ExpiresIn))
		})
	}
}
