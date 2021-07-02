package config_test

import (
	"strings"
	"testing"

	"github.com/telia-oss/sidecred"
	"github.com/telia-oss/sidecred/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestV1Config(t *testing.T) {
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
      config:
        role_arn: arn:aws:iam::role/role-name
            `),
			expected:                `requests[0]: creds[1]: duplicated request {store:secretsmanager name:open-source-dev-read-only}`,
			expectedRequestCount:    1,
			expectedCountPerRequest: []int{2},
		},
		{
			description: "errors invalid provider config",
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
      name: credential-name
      config:
        role_arn: ''
            `),
			expected:                `requests[0]: creds[0]: invalid config: "role_arn" must be defined`,
			expectedRequestCount:    1,
			expectedCountPerRequest: []int{1},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			var (
				cfg    sidecred.Config
				actual string
				err    error
			)

			cfg, err = config.Parse([]byte(tc.config))
			require.NoError(t, err)

			err = cfg.Validate()
			if err != nil {
				actual = err.Error()
			}
			assert.Equal(t, tc.expected, actual)
			assert.Equal(t, tc.expectedRequestCount, len(cfg.Requests()))
			for i, expectedCount := range tc.expectedCountPerRequest {
				assert.Equal(t, expectedCount, len(cfg.Requests()[i].Credentials))
			}
		})
	}
}
