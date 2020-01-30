package sidecred_test

import (
	"testing"
	"time"

	"github.com/telia-oss/sidecred"
	"github.com/telia-oss/sidecred/store/inprocess"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

var (
	testCredentialType = sidecred.CredentialType("fake")
	testStateID        = "fake.state.id"
	testTime           = time.Now().Add(1 * time.Hour)
)

func TestRun(t *testing.T) {
	tests := []struct {
		description          string
		namespace            string
		resources            []*sidecred.Resource
		requests             []*sidecred.Request
		expectedSecrets      map[string]string
		expectedResources    []*sidecred.Resource
		expectedCreateCalls  int
		expectedDestroyCalls int
	}{
		{
			description: "sidecred works",
			namespace:   "team-name",
			requests: []*sidecred.Request{{
				Type: testCredentialType,
				Name: testStateID,
			}},
			expectedSecrets: map[string]string{
				"team-name.fake-credential": "fake-value",
			},
			expectedResources: []*sidecred.Resource{{
				ID:         testStateID,
				Expiration: testTime,
				InUse:      true,
			}},
			expectedCreateCalls: 1,
		},
		{
			description: "does not create credentials when they exist in state",
			namespace:   "team-name",
			resources: []*sidecred.Resource{{
				ID:         testStateID,
				Expiration: testTime,
			}},
			requests: []*sidecred.Request{{
				Type: testCredentialType,
				Name: testStateID,
			}},
			expectedSecrets: map[string]string{},
			expectedResources: []*sidecred.Resource{{
				ID:         testStateID,
				Expiration: testTime,
				InUse:      true,
			}},
			expectedCreateCalls: 0,
		},
		{
			description: "replaces expired resources",
			namespace:   "team-name",
			resources: []*sidecred.Resource{{
				ID:         testStateID,
				Expiration: time.Now(),
			}},
			expectedResources:    []*sidecred.Resource{},
			expectedDestroyCalls: 1,
		},
		{
			description: "destroys deposed resources",
			namespace:   "team-name",
			resources: []*sidecred.Resource{{
				ID:         testStateID,
				Expiration: time.Now(),
			}},
			requests: []*sidecred.Request{{
				Type: testCredentialType,
				Name: testStateID,
			}},
			expectedResources: []*sidecred.Resource{{
				ID:         testStateID,
				Expiration: testTime,
				InUse:      true,
			}},
			expectedCreateCalls:  1,
			expectedDestroyCalls: 1,
		},
		{
			description: "destroys resources that are no longer requested",
			namespace:   "team-name",
			resources: []*sidecred.Resource{{
				ID:         "other.state.id",
				Expiration: testTime,
			}},
			requests:             []*sidecred.Request{},
			expectedResources:    []*sidecred.Resource{},
			expectedDestroyCalls: 1,
		},
		{
			description:     "does nothing if there are no requests",
			namespace:       "team-name",
			expectedSecrets: map[string]string{},
		},
		{
			description: "does nothing if there are no providers for the request",
			namespace:   "team-name",
			resources:   []*sidecred.Resource{},
			requests: []*sidecred.Request{{
				Type: sidecred.AWSSTS,
				Name: testStateID,
			}},
			expectedSecrets:   map[string]string{},
			expectedResources: []*sidecred.Resource{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			var (
				store    = inprocess.New()
				state    = sidecred.NewState()
				provider = &fakeProvider{}
				logger   = zaptest.NewLogger(t)
			)
			for _, r := range tc.resources {
				state.AddResource(provider.Type(), r)
			}

			s, err := sidecred.New([]sidecred.Provider{provider}, store, logger)
			require.NoError(t, err)

			err = s.Process(tc.namespace, tc.requests, state)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedCreateCalls, provider.CreateCallCount(), "create calls")
			assert.Equal(t, tc.expectedDestroyCalls, provider.DestroyCallCount(), "destroy calls")

			for _, p := range state.Providers {
				assert.Equal(t, tc.expectedResources, p.Resources)
			}

			for k, v := range tc.expectedSecrets {
				value, found, err := store.Read(k)
				assert.NoError(t, err)
				assert.True(t, found, "secret exists")
				assert.Equal(t, v, value)
			}
		})
	}
}

// Fake implementation of sidecred.Provider.
type fakeProvider struct {
	createCallCount  int
	destroyCallCount int
}

func (f *fakeProvider) Type() sidecred.ProviderType {
	return sidecred.ProviderType("fake")
}

func (f *fakeProvider) Create(r *sidecred.Request) ([]*sidecred.Credential, *sidecred.Metadata, error) {
	f.createCallCount++
	return []*sidecred.Credential{{
			Name:       "fake-credential",
			Value:      "fake-value",
			Expiration: testTime,
		}},
		nil,
		nil
}

func (f *fakeProvider) Destroy(r *sidecred.Resource) error {
	f.destroyCallCount++
	return nil
}

func (f *fakeProvider) CreateCallCount() int {
	return f.createCallCount
}

func (f *fakeProvider) DestroyCallCount() int {
	return f.destroyCallCount
}

// Fake implementation of sidecred.StateBackend.
type fakeStateBackend struct {
	state *sidecred.State
}

func (f *fakeStateBackend) Load() (*sidecred.State, error) {
	return f.state, nil
}

func (f *fakeStateBackend) Save(state *sidecred.State) error {
	f.state = state
	return nil
}
