package github_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/go-github/v45/github"
	"github.com/stretchr/testify/assert"
	"github.com/telia-oss/githubapp"
	"go.uber.org/zap/zaptest"

	"github.com/telia-oss/sidecred"
	secretstore "github.com/telia-oss/sidecred/store/github"
	"github.com/telia-oss/sidecred/store/github/githubfakes"
)

var installationToken = &github.InstallationToken{Token: github.String("access-token")}

func TestWrite(t *testing.T) {
	var (
		teamName       = "team-name"
		secret         = &sidecred.Credential{Name: "secret-name", Value: "secret-value"}
		secretTemplate = "concourse_{{ .Namespace }}_{{ .Name }}"
	)

	tests := []struct {
		description                 string
		config                      json.RawMessage
		secretTemplate              string
		secretPath                  string
		expectedError               error
		expectedCreateOrUpdateCalls int
		expectedGetPublicKeyCalls   int
	}{
		{
			description:                 "github repository secrets works",
			config:                      []byte(`{"repository":"owner/repository"}`),
			secretTemplate:              secretTemplate,
			secretPath:                  "CONCOURSE_TEAM_NAME_SECRET_NAME",
			expectedGetPublicKeyCalls:   1,
			expectedCreateOrUpdateCalls: 1,
		},
		{
			description:                 "supports setting secret template from config",
			config:                      []byte(`{"repository":"owner/repository","secret_template":"SIDECRED_{{ .Namespace }}_{{ .Name }}"}`),
			secretTemplate:              secretTemplate,
			secretPath:                  "SIDECRED_TEAM_NAME_SECRET_NAME",
			expectedGetPublicKeyCalls:   1,
			expectedCreateOrUpdateCalls: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			fakeApp := &githubfakes.FakeApp{}
			fakeApp.CreateInstallationTokenReturns(&githubapp.Token{InstallationToken: installationToken}, nil)

			fakeActionsAPI := &githubfakes.FakeActionsAPI{}
			fakeActionsAPI.CreateOrUpdateRepoSecretReturns(nil, nil)

			store := secretstore.NewStore(fakeApp,
				zaptest.NewLogger(t),
				secretstore.WithSecretTemplate(tc.secretTemplate),
				secretstore.WithActionsClientFactory(func(string) secretstore.ActionsAPI {
					return fakeActionsAPI
				}),
			)
			path, err := store.Write(context.TODO(), teamName, secret, tc.config)

			assert.Equal(t, tc.expectedError, err)
			assert.Equal(t, tc.secretPath, path)
		})
	}
}

func TestRead(t *testing.T) {
	var (
		secretPath  = "CONCOURSE_TEAM_NAME_SECRET_NAME"
		secretValue = "secret-value"
	)

	tests := []struct {
		description    string
		config         json.RawMessage
		secretPath     string
		getSecretError error
		expectedSecret string
		expectFound    bool
		expectedError  error
	}{
		{
			description:    "works as expected",
			config:         []byte(`{"repository":"owner/repository"}`),
			secretPath:     secretPath,
			expectedSecret: secretValue,
			expectFound:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			fakeApp := &githubfakes.FakeApp{}
			fakeApp.CreateInstallationTokenReturns(&githubapp.Token{InstallationToken: installationToken}, nil)

			fakeActionsAPI := &githubfakes.FakeActionsAPI{}
			fakeActionsAPI.GetRepoSecretReturns(&github.Secret{Name: secretValue}, nil, nil)

			store := secretstore.NewStore(fakeApp,
				zaptest.NewLogger(t),
				secretstore.WithActionsClientFactory(func(string) secretstore.ActionsAPI {
					return fakeActionsAPI
				}),
			)
			secret, found, err := store.Read(context.TODO(), tc.secretPath, tc.config)

			assert.Equal(t, tc.expectedError, err)
			assert.Equal(t, tc.expectFound, found)
			assert.Equal(t, tc.expectedSecret, secret)
		})
	}
}

func TestDelete(t *testing.T) {
	secretPath := "CONCOURSE_TEAM_NAME_SECRET_NAME"

	tests := []struct {
		description       string
		config            json.RawMessage
		secretPath        string
		deleteSecretError error
		expectedError     error
	}{
		{
			description: "works as expected",
			config:      []byte(`{"repository":"owner/repository"}`),
			secretPath:  secretPath,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			fakeApp := &githubfakes.FakeApp{}
			fakeApp.CreateInstallationTokenReturns(&githubapp.Token{InstallationToken: installationToken}, nil)

			fakeActionsAPI := &githubfakes.FakeActionsAPI{}
			fakeActionsAPI.DeleteRepoSecretReturns(nil, nil)

			store := secretstore.NewStore(fakeApp,
				zaptest.NewLogger(t),
				secretstore.WithActionsClientFactory(func(string) secretstore.ActionsAPI {
					return fakeActionsAPI
				}),
			)
			err := store.Delete(context.TODO(), tc.secretPath, tc.config)

			assert.Equal(t, tc.expectedError, err)
			assert.Equal(t, 1, fakeActionsAPI.DeleteRepoSecretCallCount())
		})
	}
}
