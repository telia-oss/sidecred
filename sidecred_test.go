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
	testCredentialType = sidecred.Randomized
	testStateID        = "fake.state.id"
	testTime           = time.Now().Add(1 * time.Hour)
)

func TestProcess(t *testing.T) {
	tests := []struct {
		description          string
		namespace            string
		resources            []*sidecred.Resource
		requests             []*sidecred.CredentialRequest
		expectedSecrets      map[string]string
		expectedResources    []*sidecred.Resource
		expectedCreateCalls  int
		expectedDestroyCalls int
	}{
		{
			description: "sidecred works",
			namespace:   "team-name",
			requests: []*sidecred.CredentialRequest{{
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
			requests: []*sidecred.CredentialRequest{{
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
			description: "replaces expired resources (within the rotation window)",
			namespace:   "team-name",
			resources: []*sidecred.Resource{{
				ID:         testStateID,
				Expiration: time.Now().Add(3 * time.Minute),
			}},
			requests: []*sidecred.CredentialRequest{{
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
			description: "destroys deposed resources",
			namespace:   "team-name",
			resources: []*sidecred.Resource{{
				ID:         testStateID,
				Expiration: time.Now(),
			}},
			requests: []*sidecred.CredentialRequest{{
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
			requests:             []*sidecred.CredentialRequest{},
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
			requests: []*sidecred.CredentialRequest{{
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

			s, err := sidecred.New([]sidecred.Provider{provider}, []sidecred.SecretStore{store}, 10*time.Minute, logger)
			require.NoError(t, err)

			err = s.Process(newConfig(tc.namespace, tc.requests), state)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedCreateCalls, provider.CreateCallCount(), "create calls")
			assert.Equal(t, tc.expectedDestroyCalls, provider.DestroyCallCount(), "destroy calls")

			for _, p := range state.Providers {
				assert.Equal(t, tc.expectedResources, p.Resources)
			}

			for k, v := range tc.expectedSecrets {
				value, found, err := store.Read(k, sidecred.NoConfig)
				assert.NoError(t, err)
				assert.True(t, found, "secret exists")
				assert.Equal(t, v, value)
			}
		})
	}
}

// This test exists because looping over pointers as done when cleaning up expired/deposed
// resources (and deposed secrets) can lead to surprising behaviours. The test below ensures
// that things are working as intended.
func TestProcessCleanup(t *testing.T) {
	tests := []struct {
		description          string
		namespace            string
		resources            []*sidecred.Resource
		secrets              []*sidecred.Secret
		expectedDestroyCalls int
	}{
		{
			description: "cleanup works",
			namespace:   "team-name",
			resources: []*sidecred.Resource{
				{
					ID:         "r1",
					Expiration: time.Now(),
				},
				{
					ID:         "r2",
					Expiration: time.Now(),
				},
				{
					ID:         "r3",
					Expiration: time.Now(),
				},
			},
			secrets: []*sidecred.Secret{
				{
					ResourceID: "r1",
					Path:       "path1",
					Expiration: time.Now(),
				},
				{
					ResourceID: "r1",
					Path:       "path2",
					Expiration: time.Now(),
				},
				{
					ResourceID: "r2",
					Path:       "path3",
					Expiration: time.Now(),
				},
			},
			expectedDestroyCalls: 3,
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

			for _, s := range tc.secrets {
				state.AddSecret(&sidecred.StoreConfig{Type: store.Type()}, s)
			}

			s, err := sidecred.New([]sidecred.Provider{provider}, []sidecred.SecretStore{store}, 10*time.Minute, logger)
			require.NoError(t, err)

			err = s.Process(newConfig(tc.namespace, []*sidecred.CredentialRequest{}), state)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedDestroyCalls, provider.DestroyCallCount(), "destroy calls")

			for _, p := range state.Providers {
				if !assert.Equal(t, 0, len(p.Resources)) {
					for _, s := range p.Resources {
						assert.Nil(t, s)
					}
				}
			}

			for _, p := range state.Stores {
				if !assert.Equal(t, 0, len(p.Secrets)) {
					for _, s := range p.Secrets {
						assert.Nil(t, s)
					}
				}
			}
		})
	}
}

func newConfig(namespace string, requests []*sidecred.CredentialRequest) *sidecred.Config {
	var re []*sidecred.CredentialRequestConfig
	for _, r := range requests {
		re = append(re, &sidecred.CredentialRequestConfig{
			CredentialRequest: r,
		})
	}
	return &sidecred.Config{
		Version:   1,
		Namespace: namespace,
		Stores: []*sidecred.StoreConfig{{
			Type: sidecred.Inprocess,
		}},
		Requests: []*sidecred.RequestConfig{{
			Store: string(sidecred.Inprocess),
			Creds: re,
		}},
	}
}

// Fake implementation of sidecred.Provider.
type fakeProvider struct {
	createCallCount  int
	destroyCallCount int
}

func (f *fakeProvider) Type() sidecred.ProviderType {
	return sidecred.Random
}

func (f *fakeProvider) Create(r *sidecred.CredentialRequest) ([]*sidecred.Credential, *sidecred.Metadata, error) {
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
