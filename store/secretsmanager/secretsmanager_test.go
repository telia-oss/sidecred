package secretsmanager_test

import (
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/stretchr/testify/assert"

	"github.com/telia-oss/sidecred"
	secretstore "github.com/telia-oss/sidecred/store/secretsmanager"
	"github.com/telia-oss/sidecred/store/secretsmanager/secretsmanagerfakes"
)

func TestWrite(t *testing.T) {
	var (
		teamName       = "team-name"
		secret         = &sidecred.Credential{Name: "secret-name", Value: "secret-value"}
		secretTemplate = "/concourse/{{ .Namespace }}/{{ .Name }}"
		secretPath     = "/concourse/team-name/secret-name"
	)

	tests := []struct {
		description         string
		config              json.RawMessage
		secretTemplate      string
		secretPath          string
		createError         error
		updateError         error
		expectedError       error
		expectedCreateCalls int
		expectedUpdateCalls int
	}{
		{
			description:         "secretsmanager store works",
			secretTemplate:      secretTemplate,
			secretPath:          secretPath,
			expectedCreateCalls: 1,
			expectedUpdateCalls: 1,
		},
		{
			description:         "supports arbitrary path templates",
			secretTemplate:      "concourse.{{ .Namespace }}.{{ .Name }}",
			secretPath:          "concourse.team-name.secret-name",
			expectedCreateCalls: 1,
			expectedUpdateCalls: 1,
		},
		{
			description:         "does not error if the secret already exists",
			secretTemplate:      secretTemplate,
			secretPath:          secretPath,
			createError:         awserr.New(secretsmanager.ErrCodeResourceExistsException, "", nil),
			expectedError:       nil,
			expectedCreateCalls: 1,
			expectedUpdateCalls: 1,
		},
		{
			description:         "propagates errors when creating the secret",
			secretTemplate:      secretTemplate,
			secretPath:          "",
			createError:         awserr.New("failure", "", nil),
			expectedError:       awserr.New("failure", "", nil),
			expectedCreateCalls: 1,
		},
		{
			description:         "propagates errors when updating the secret",
			secretTemplate:      secretTemplate,
			updateError:         awserr.New("failure", "", nil),
			expectedError:       awserr.New("failure", "", nil),
			expectedCreateCalls: 1,
			expectedUpdateCalls: 1,
		},
		{
			description:         "supports setting secret template from config",
			config:              []byte(`{"secret_template":"{{ .Namespace }}!?!{{ .Name }}"}`),
			secretTemplate:      secretTemplate,
			secretPath:          "team-name!?!secret-name",
			expectedCreateCalls: 1,
			expectedUpdateCalls: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			client := &secretsmanagerfakes.FakeSecretsManagerAPI{}
			client.CreateSecretReturns(nil, tc.createError)
			client.UpdateSecretReturns(nil, tc.updateError)

			store := secretstore.New(client, secretstore.WithSecretTemplate(tc.secretTemplate))
			path, err := store.Write(teamName, secret, tc.config)

			assert.Equal(t, tc.expectedError, err)
			assert.Equal(t, tc.secretPath, path)
			assert.Equal(t, tc.expectedCreateCalls, client.CreateSecretCallCount())
			assert.Equal(t, tc.expectedUpdateCalls, client.UpdateSecretCallCount())
		})
	}
}

func TestRead(t *testing.T) {
	var (
		secretPath  = "/concourse/team-name/secret-name"
		secretValue = "secret-value"
	)

	tests := []struct {
		description     string
		secretPath      string
		getSecretOutput *secretsmanager.GetSecretValueOutput
		getSecretError  error
		expectedSecret  string
		expectFound     bool
		expectedError   error
	}{
		{
			description: "works as expected",
			secretPath:  secretPath,
			getSecretOutput: &secretsmanager.GetSecretValueOutput{
				SecretString: aws.String(secretValue),
			},
			expectedSecret: secretValue,
			expectFound:    true,
		},
		{
			description:    "returns false if the secret does not exist",
			secretPath:     secretPath,
			getSecretError: awserr.New(secretsmanager.ErrCodeResourceNotFoundException, "", nil),
			expectFound:    false,
		},
		{
			description:    "propagates aws errors",
			secretPath:     secretPath,
			getSecretError: awserr.New("failure", "", nil),
			expectFound:    false,
			expectedError:  awserr.New("failure", "", nil),
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			client := &secretsmanagerfakes.FakeSecretsManagerAPI{}
			client.GetSecretValueReturns(tc.getSecretOutput, tc.getSecretError)

			store := secretstore.New(client)
			secret, found, err := store.Read(tc.secretPath, nil)

			assert.Equal(t, tc.expectedError, err)
			assert.Equal(t, tc.expectFound, found)
			assert.Equal(t, tc.expectedSecret, secret)
			assert.Equal(t, 1, client.GetSecretValueCallCount())
		})
	}
}

func TestDelete(t *testing.T) {
	var (
		secretPath = "/concourse/team-name/secret-name"
	)

	tests := []struct {
		description       string
		secretPath        string
		deleteSecretError error
		expectedError     error
	}{
		{
			description: "works as expected",
			secretPath:  secretPath,
		},
		{
			description:       "ignores error if secret does not exist",
			secretPath:        secretPath,
			deleteSecretError: awserr.New(secretsmanager.ErrCodeResourceNotFoundException, "", nil),
			expectedError:     nil,
		},
		{
			description:       "propagates aws errors",
			secretPath:        secretPath,
			deleteSecretError: awserr.New("failure", "", nil),
			expectedError:     awserr.New("failure", "", nil),
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			client := &secretsmanagerfakes.FakeSecretsManagerAPI{}
			client.DeleteSecretReturns(nil, tc.deleteSecretError)

			store := secretstore.New(client)
			err := store.Delete(tc.secretPath, nil)

			assert.Equal(t, tc.expectedError, err)
			assert.Equal(t, 1, client.DeleteSecretCallCount())
		})
	}
}
