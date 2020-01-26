package inprocess_test

import (
	"testing"

	"github.com/telia-oss/sidecred"
	secretstore "github.com/telia-oss/sidecred/store/inprocess"

	"github.com/stretchr/testify/assert"
)

func TestInProcessStore(t *testing.T) {
	var (
		teamName     = "team-name"
		secret       = &sidecred.Credential{Name: "secret-name", Value: "secret-value"}
		pathTemplate = "/concourse/{{ .Namespace }}/{{ .Name }}"
		secretPath   = "/concourse/team-name/secret-name"
	)

	tests := []struct {
		description  string
		pathTemplate string
		secretPath   string
	}{
		{
			description:  "works as expected",
			pathTemplate: pathTemplate,
			secretPath:   secretPath,
		},
		{
			description:  "supports arbitrary path templates",
			pathTemplate: "concourse.{{ .Namespace }}.{{ .Name }}",
			secretPath:   "concourse.team-name.secret-name",
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			store := secretstore.New(secretstore.WithPathTemplate(tc.pathTemplate))

			path, err := store.Write(teamName, secret)
			assert.NoError(t, err)
			assert.Equal(t, tc.secretPath, path)

			actual, found, err := store.Read(path)
			assert.Nil(t, err)
			assert.Equal(t, true, found)
			assert.Equal(t, secret.Value, actual)

			err = store.Delete(path)
			assert.Nil(t, err)
		})
	}
}
