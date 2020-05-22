package github_test

import (
	"testing"
	"time"

	"github.com/telia-oss/githubapp"
	"github.com/telia-oss/sidecred"
	provider "github.com/telia-oss/sidecred/provider/github"
	"github.com/telia-oss/sidecred/provider/github/githubfakes"

	"github.com/google/go-github/v29/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGithubProvider(t *testing.T) {
	var (
		targetTime  = time.Date(2020, 1, 29, 4, 29, 0, 0, time.UTC)
		targetToken = &github.InstallationToken{Token: github.String("access-token")}
		targetKey   = &github.Key{
			ID:        github.Int64(1),
			CreatedAt: &github.Timestamp{Time: targetTime},
		}
	)
	tests := []struct {
		description      string
		request          *sidecred.Request
		expected         []*sidecred.Credential
		expectedMetadata *sidecred.Metadata
	}{
		{
			description: "Github provider works for deploy keys",
			request: &sidecred.Request{
				Type:   sidecred.GithubDeployKey,
				Name:   "request-name",
				Config: []byte(`{"owner":"request-owner","repository":"request-repository","title":"request-title","read_only":true}`),
			},
			expected: []*sidecred.Credential{{
				Name:        "request-repository-deploy-key",
				Description: "Github deploy key managed by sidecred.",
			}},
			expectedMetadata: &sidecred.Metadata{"key_id": "1"},
		},
		{
			description: "Github provider works for access tokens",
			request: &sidecred.Request{
				Type:   sidecred.GithubAccessToken,
				Name:   "request-name",
				Config: []byte(`{"owner":"request-owner"}`),
			},
			expected: []*sidecred.Credential{{
				Name:        "request-owner-access-token",
				Value:       "access-token",
				Description: "Github access token managed by sidecred.",
			}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			fakeApp := &githubfakes.FakeApp{}
			fakeApp.CreateInstallationTokenReturns(&githubapp.Token{InstallationToken: targetToken}, nil)

			fakeReposAPI := &githubfakes.FakeRepositoriesAPI{}
			fakeReposAPI.CreateKeyReturns(targetKey, nil, nil)

			p := provider.New(fakeApp,
				provider.WithReposClientFactory(func(string) provider.RepositoriesAPI {
					return fakeReposAPI
				}),
			)

			creds, metadata, err := p.Create(tc.request)
			require.NoError(t, err)
			require.Len(t, creds, len(tc.expected))
			assert.Equal(t, tc.expectedMetadata, metadata)

			for i, e := range tc.expected {
				assert.Equal(t, e.Name, creds[i].Name)
				assert.Equal(t, e.Description, creds[i].Description)
				if tc.request.Type != sidecred.GithubDeployKey {
					assert.Equal(t, e.Value, creds[i].Value)
				}
			}
		})
	}
}
