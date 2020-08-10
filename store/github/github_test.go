package github_test

import (
	"testing"

	"github.com/telia-oss/sidecred"
	secretstore "github.com/telia-oss/sidecred/store/github"
	"github.com/telia-oss/sidecred/store/github/githubfakes"

	"github.com/google/go-github/v29/github"
	"github.com/stretchr/testify/assert"
	"github.com/telia-oss/githubapp"
)

var (
	installationToken = &github.InstallationToken{Token: github.String("access-token")}
)

func TestWrite(t *testing.T) {
	var (
		teamName     = "team-name"
		secret       = &sidecred.Credential{Name: "secret-name", Value: "secret-value"}
		pathTemplate = "concourse_{{ .Namespace }}_{{ .Name }}"
	)

	tests := []struct {
		description                 string
		pathTemplate                string
		expectedPath                string
		expectedError               error
		expectedCreateOrUpdateCalls int
		expectedGetPublicKeyCalls   int
	}{
		{
			description:                 "github repository secrets works",
			pathTemplate:                pathTemplate,
			expectedPath:                "CONCOURSE_TEAM_NAME_SECRET_NAME",
			expectedGetPublicKeyCalls:   1,
			expectedCreateOrUpdateCalls: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			fakeApp := &githubfakes.FakeApp{}
			fakeApp.CreateInstallationTokenReturns(&githubapp.Token{InstallationToken: installationToken}, nil)

			fakeActionsAPI := &githubfakes.FakeActionsAPI{}
			fakeActionsAPI.CreateOrUpdateSecretReturns(nil, nil)

			store := secretstore.New(fakeApp,
				"repository",
				"owner",
				secretstore.WithSecretTemplate(tc.pathTemplate),
				secretstore.WithActionsClientFactory(func(string) secretstore.ActionsAPI {
					return fakeActionsAPI
				}),
			)
			path, err := store.Write(teamName, secret)

			assert.Equal(t, tc.expectedError, err)
			assert.Equal(t, tc.expectedPath, path)
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
		secretPath     string
		getSecretError error
		expectedSecret string
		expectFound    bool
		expectedError  error
	}{
		{
			description:    "works as expected",
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
			fakeActionsAPI.GetSecretReturns(&github.Secret{Name: secretValue}, nil, nil)

			store := secretstore.New(fakeApp,
				"repository",
				"owner",
				secretstore.WithActionsClientFactory(func(string) secretstore.ActionsAPI {
					return fakeActionsAPI
				}),
			)
			secret, found, err := store.Read(tc.secretPath)

			assert.Equal(t, tc.expectedError, err)
			assert.Equal(t, tc.expectFound, found)
			assert.Equal(t, tc.expectedSecret, secret)
		})
	}
}

func TestDelete(t *testing.T) {
	var (
		secretPath = "CONCOURSE_TEAM_NAME_SECRET_NAME"
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
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			fakeApp := &githubfakes.FakeApp{}
			fakeApp.CreateInstallationTokenReturns(&githubapp.Token{InstallationToken: installationToken}, nil)

			fakeActionsAPI := &githubfakes.FakeActionsAPI{}
			fakeActionsAPI.DeleteSecretReturns(nil, nil)

			store := secretstore.New(fakeApp,
				"owner",
				"repository",
				secretstore.WithActionsClientFactory(func(string) secretstore.ActionsAPI {
					return fakeActionsAPI
				}),
			)
			err := store.Delete(tc.secretPath)

			assert.Equal(t, tc.expectedError, err)
			assert.Equal(t, 1, fakeActionsAPI.DeleteSecretCallCount())
		})
	}
}
