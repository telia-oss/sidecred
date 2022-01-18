package random_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/telia-oss/sidecred"
	provider "github.com/telia-oss/sidecred/provider/random"
)

func TestRandomProvider(t *testing.T) {
	tests := []struct {
		description string
		seed        int64
		request     *sidecred.CredentialRequest
		expected    []*sidecred.Credential
	}{
		{
			description: "random provider works",
			seed:        1,
			request: &sidecred.CredentialRequest{
				Type: sidecred.Randomized,
				Name: "request-name",
			},
			expected: []*sidecred.Credential{{
				Name:        "request-name",
				Value:       "",
				Description: "Random generated secret managed by Sidecred.",
			}},
		},
		{
			description: "we can set the length of the random string",
			seed:        1,
			request: &sidecred.CredentialRequest{
				Type:   sidecred.Randomized,
				Name:   "request-name",
				Config: []byte(`{"length":5}`),
			},
			expected: []*sidecred.Credential{{
				Name:        "request-name",
				Value:       "1TrAn",
				Description: "Random generated secret managed by Sidecred.",
			}},
		},
		{
			description: "we can control the seed",
			seed:        2,
			request: &sidecred.CredentialRequest{
				Type:   sidecred.Randomized,
				Name:   "request-name",
				Config: []byte(`{"length":5}`),
			},
			expected: []*sidecred.Credential{{
				Name:        "request-name",
				Value:       "bsviM",
				Description: "Random generated secret managed by Sidecred.",
			}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			p := provider.New(tc.seed)

			creds, metadata, err := p.Create(tc.request)
			require.NoError(t, err)
			require.Len(t, creds, len(tc.expected))
			assert.Nil(t, metadata)

			for i, e := range tc.expected {
				assert.Equal(t, e.Name, creds[i].Name)
				assert.Equal(t, e.Value, creds[i].Value)
				assert.Equal(t, e.Description, creds[i].Description)
			}
		})
	}
}
