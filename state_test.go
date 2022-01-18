package sidecred_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/telia-oss/sidecred"
)

var (
	fixedTestTime = time.Date(2020, 1, 30, 12, 0, 0, 0, time.UTC)
)

func TestState(t *testing.T) {
	tests := []struct {
		description       string
		stateID           string
		expectFound       bool
		expectedJSON      string
		expectedFinalJSON string
	}{
		{
			description: "state works",
			stateID:     testStateID,
			expectedJSON: strings.TrimSpace(`
{"providers":[{"type":"random","resources":[{"type":"random","id":"fake.state.id","store":"","expiration":"2020-01-30T12:00:00Z","deposed":false}]}],"stores":[{"type":"inprocess","name":"","secrets":[{"resource_id":"fake.state.id","path":"fake.store.path","expiration":"2020-01-30T12:00:00Z"}]}]}
`),
			expectedFinalJSON: strings.TrimSpace(`
{"providers":[{"type":"random","resources":[]}],"stores":[{"type":"inprocess","name":"","secrets":[]}]}
`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			state := sidecred.NewState()

			state.AddResource(&sidecred.Resource{
				Type:       sidecred.Randomized,
				ID:         tc.stateID,
				Expiration: fixedTestTime,
			})
			storeConfig := &sidecred.StoreConfig{Type: sidecred.Inprocess}
			state.AddSecret(storeConfig, &sidecred.Secret{
				ResourceID: tc.stateID,
				Path:       "fake.store.path",
				Expiration: fixedTestTime,
			})

			outputJSON, err := json.Marshal(state)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedJSON, string(outputJSON))

			state.RemoveResource(&sidecred.Resource{Type: sidecred.Randomized, ID: tc.stateID})
			state.RemoveSecret(storeConfig, &sidecred.Secret{Path: "fake.store.path"})

			finalOutputJSON, err := json.Marshal(state)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedFinalJSON, string(finalOutputJSON))
		})
	}
}
