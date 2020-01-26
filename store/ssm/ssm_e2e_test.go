// +build e2e

package ssm_test

import (
	"fmt"
	"testing"

	"github.com/telia-oss/sidecred"
	secretstore "github.com/telia-oss/sidecred/store/ssm"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecretsManagerStoreE2E(t *testing.T) {
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
			description:  "ssm parameter store works",
			pathTemplate: "/sidecred/{{ .Namespace }}/{{ .Name }}",
			expectedPath: fmt.Sprintf("/sidecred/%s/%s", namespace, secretName),
		},
		{
			description:  "supports arbitrary path templates",
			pathTemplate: "sidecred.{{ .Namespace }}.{{ .Name }}",
			expectedPath: fmt.Sprintf("sidecred.%s.%s", namespace, secretName),
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			sess, err := session.NewSession(&aws.Config{Region: aws.String("eu-west-1")})
			require.NoError(t, err)

			store := secretstore.New(
				secretstore.NewClient(sess),
				secretstore.WithPathTemplate(tc.pathTemplate),
			)

			path, err := store.Write(namespace, &sidecred.Credential{
				Name:  secretName,
				Value: secretValue,
			})
			require.NoError(t, err)
			assert.Equal(t, tc.expectedPath, path)

			value, found, err := store.Read(path)
			assert.NoError(t, err, "read secret")
			assert.True(t, found, "found secret")
			assert.Equal(t, secretValue, value)

			if err := store.Delete(path); err != nil {
				t.Errorf("delete secret (%s): %s", path, err)
			}
		})
	}
}
