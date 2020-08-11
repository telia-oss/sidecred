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
		description string
		config      string
		expected    string
	}{
		{
			description: "works",
			config: strings.TrimSpace(`
---
version: 1
namespace: cloudops

requests:
  - type: aws:sts
    name: open-source-dev-read-only
    config:
      role_arn: arn:aws:iam::role/role-name
      duration: 900
            `),
			expected: "",
		},
		{
			description: "errors on duplicate requests",
			config: strings.TrimSpace(`
---
version: 1
namespace: cloudops

requests:
  - type: aws:sts
    name: open-source-dev-read-only
    config:
      role_arn: arn:aws:iam::role/role-name
      duration: 900
  - type: aws:sts
    name: open-source-dev-read-only
            `),
			expected: `requests[1]: duplicate request "open-source-dev-read-only"`,
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
		})
	}
}
