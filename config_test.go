package sidecred_test

import (
	"strings"
	"testing"

	"github.com/telia-oss/sidecred"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestConfig(t *testing.T) {
	tests := []struct {
		description             string
		config                  string
		expected                string
		expectedRequestCount    int
		expectedCountPerRequest []int
	}{
		{
			description: "works",
			config: strings.TrimSpace(`
---
version: 1
namespace: cloudops

stores:
  - type: secretsmanager

requests:
  - store: secretsmanager
    creds:
    - type: aws:sts
      name: open-source-dev-read-only
      config:
        role_arn: arn:aws:iam::role/role-name
        duration: 15m
            `),
			expected:                "",
			expectedRequestCount:    1,
			expectedCountPerRequest: []int{1},
		},
		{
			description: "supports request lists",
			config: strings.TrimSpace(`
---
version: 1
namespace: cloudops

stores:
  - type: secretsmanager

requests:
  - store: secretsmanager
    creds:
    - type: aws:sts
      list:
      - name: open-source-dev-read-only
        config:
          role_arn: arn:aws:iam::role/role-name
          duration: 15m
            `),
			expected:                "",
			expectedRequestCount:    1,
			expectedCountPerRequest: []int{1},
		},
		{
			description: "supports named stores",
			config: strings.TrimSpace(`
---
version: 1
namespace: cloudops

stores:
  - type: secretsmanager
  - type: secretsmanager
    name: concourse

requests:
  - store: concourse
    creds:
    - type: aws:sts
      list:
      - name: open-source-dev-read-only
        config:
          role_arn: arn:aws:iam::role/role-name
          duration: 15m
            `),
			expected:                "",
			expectedRequestCount:    1,
			expectedCountPerRequest: []int{1},
		},
		{
			description: "errors on duplicate requests",
			config: strings.TrimSpace(`
---
version: 1
namespace: cloudops

stores:
  - type: secretsmanager
  - type: inprocess

requests:
  - store: secretsmanager
    creds:
    - type: aws:sts
      name: open-source-dev-read-only
      config:
        role_arn: arn:aws:iam::role/role-name
        duration: 15m
    - type: aws:sts
      name: open-source-dev-read-only
            `),
			expected:                `requests[0]: creds[1]: duplicated request {store:secretsmanager name:open-source-dev-read-only}`,
			expectedRequestCount:    1,
			expectedCountPerRequest: []int{2},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			var (
				config *sidecred.Config
				actual string
				err    error
			)

			err = yaml.Unmarshal([]byte(tc.config), &config)
			require.NoError(t, err)

			err = config.Validate()
			if err != nil {
				actual = err.Error()
			}
			assert.Equal(t, tc.expected, actual)
			assert.Equal(t, tc.expectedRequestCount, len(config.Requests))
			for i, expectedCount := range tc.expectedCountPerRequest {
				assert.Equal(t, expectedCount, len(config.Requests[i].CredentialRequests()))
			}
		})
	}
}
