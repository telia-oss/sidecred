package ssm_test

import (
	"encoding/json"
	"testing"

	"github.com/telia-oss/sidecred"
	secretstore "github.com/telia-oss/sidecred/store/ssm"
	"github.com/telia-oss/sidecred/store/ssm/ssmfakes"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/assert"
)

func TestWrite(t *testing.T) {
	var (
		teamName       = "team-name"
		secret         = &sidecred.Credential{Name: "secret-name", Value: "secret-value"}
		secretTemplate = "/concourse/{{ .Namespace }}/{{ .Name }}"
		secretPath     = "/concourse/team-name/secret-name"
	)

	tests := []struct {
		description    string
		config         json.RawMessage
		secretTemplate string
		secretPath     string
		putError       error
		expectedError  error
	}{
		{
			description:    "ssm parameter store works",
			secretTemplate: secretTemplate,
			secretPath:     secretPath,
		},
		{
			description:    "supports arbitrary path templates",
			secretTemplate: "concourse.{{ .Namespace }}.{{ .Name }}",
			secretPath:     "concourse.team-name.secret-name",
		},
		{
			description:    "propagates aws errors",
			secretTemplate: secretTemplate,
			secretPath:     "",
			putError:       awserr.New("failure", "", nil),
			expectedError:  awserr.New("failure", "", nil),
		},
		{
			description:    "supports setting secret template from config",
			config:         []byte(`{"secret_template":"{{ .Namespace }}!?!{{ .Name }}"}`),
			secretTemplate: secretTemplate,
			secretPath:     "team-name!?!secret-name",
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			client := &ssmfakes.FakeSSMAPI{}
			client.PutParameterReturns(nil, tc.putError)

			store := secretstore.New(client, secretstore.WithSecretTemplate(tc.secretTemplate))
			path, err := store.Write(teamName, secret, tc.config)

			assert.Equal(t, tc.expectedError, err)
			assert.Equal(t, tc.secretPath, path)
			assert.Equal(t, 1, client.PutParameterCallCount())
		})
	}
}

func TestRead(t *testing.T) {
	var (
		secretPath  = "/concourse/team-name/secret-name"
		secretValue = "secret-value"
	)

	tests := []struct {
		description        string
		secretPath         string
		getParameterOutput *ssm.GetParameterOutput
		getParameterError  error
		expectedSecret     string
		expectFound        bool
		expectedError      error
	}{
		{
			description: "works as expected",
			secretPath:  secretPath,
			getParameterOutput: &ssm.GetParameterOutput{
				Parameter: &ssm.Parameter{
					Value: aws.String(secretValue),
				},
			},
			expectedSecret: secretValue,
			expectFound:    true,
		},
		{
			description:       "returns false if the secret does not exist",
			secretPath:        secretPath,
			getParameterError: awserr.New(ssm.ErrCodeParameterNotFound, "", nil),
			expectFound:       false,
		},
		{
			description:       "propagates aws errors",
			secretPath:        secretPath,
			getParameterError: awserr.New("failure", "", nil),
			expectFound:       false,
			expectedError:     awserr.New("failure", "", nil),
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			client := &ssmfakes.FakeSSMAPI{}
			client.GetParameterReturns(tc.getParameterOutput, tc.getParameterError)

			store := secretstore.New(client)
			secret, found, err := store.Read(tc.secretPath, nil)

			assert.Equal(t, tc.expectedError, err)
			assert.Equal(t, tc.expectFound, found)
			assert.Equal(t, tc.expectedSecret, secret)
			assert.Equal(t, 1, client.GetParameterCallCount())
		})
	}
}

func TestDelete(t *testing.T) {
	var (
		secretPath = "/concourse/team-name/secret-name"
	)

	tests := []struct {
		description          string
		secretPath           string
		deleteParameterError error
		expectedError        error
	}{
		{
			description: "works as expected",
			secretPath:  secretPath,
		},
		{
			description:          "ignores error if parameter does not exist",
			secretPath:           secretPath,
			deleteParameterError: awserr.New(ssm.ErrCodeParameterNotFound, "", nil),
			expectedError:        nil,
		},
		{
			description:          "propagates aws errors",
			secretPath:           secretPath,
			deleteParameterError: awserr.New("failure", "", nil),
			expectedError:        awserr.New("failure", "", nil),
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			client := &ssmfakes.FakeSSMAPI{}
			client.DeleteParameterReturns(nil, tc.deleteParameterError)

			store := secretstore.New(client)
			err := store.Delete(tc.secretPath, nil)

			assert.Equal(t, tc.expectedError, err)
			assert.Equal(t, 1, client.DeleteParameterCallCount())
		})
	}
}
