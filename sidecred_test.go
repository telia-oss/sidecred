package sidecred_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/telia-oss/sidecred"
	"github.com/telia-oss/sidecred/config"
	"github.com/telia-oss/sidecred/eventctx"
	"github.com/telia-oss/sidecred/store/inprocess"
)

var (
	testStateID = "fake.state.id"
	testTime    = time.Now().Add(1 * time.Hour)
)

func TestProcess(t *testing.T) {
	tests := []struct {
		description          string
		config               string
		resources            []*sidecred.Resource
		expectedSecrets      map[string]string
		expectedResources    []*sidecred.Resource
		expectedCreateCalls  int
		expectedDestroyCalls int
	}{
		{
			description: "sidecred works",
			config: strings.TrimSpace(`
---
version: 1
namespace: team-name

stores:
- type: inprocess

requests:
- store: inprocess
  creds:
  - type: random
    name: fake.state.id
			`),
			expectedSecrets: map[string]string{
				"team-name.fake-credential": "fake-value",
			},
			expectedResources: []*sidecred.Resource{{
				Type:       sidecred.Randomized,
				ID:         testStateID,
				Store:      "inprocess",
				Expiration: testTime,
				InUse:      true,
			}},
			expectedCreateCalls: 1,
		},
		{
			description: "does not create credentials when they exist in state",
			config: strings.TrimSpace(`
---
version: 1
namespace: team-name

stores:
- type: inprocess

requests:
- store: inprocess
  creds:
  - type: random
    name: fake.state.id
			`),
			resources: []*sidecred.Resource{{
				Type:       sidecred.Randomized,
				ID:         testStateID,
				Store:      "inprocess",
				Expiration: testTime,
			}},
			expectedSecrets: map[string]string{},
			expectedResources: []*sidecred.Resource{{
				Type:       sidecred.Randomized,
				ID:         testStateID,
				Store:      "inprocess",
				Expiration: testTime,
				InUse:      true,
			}},
			expectedCreateCalls: 0,
		},
		{
			description: "replaces expired resources (within the rotation window)",
			config: strings.TrimSpace(`
---
version: 1
namespace: team-name

stores:
- type: inprocess

requests:
- store: inprocess
  creds:
  - type: random
    name: fake.state.id
			`),
			resources: []*sidecred.Resource{{
				Type:       sidecred.Randomized,
				ID:         testStateID,
				Store:      "inprocess",
				Expiration: time.Now().Add(3 * time.Minute),
			}},
			expectedResources: []*sidecred.Resource{{
				Type:       sidecred.Randomized,
				ID:         testStateID,
				Store:      "inprocess",
				Expiration: testTime,
				InUse:      true,
			}},
			expectedCreateCalls:  1,
			expectedDestroyCalls: 1,
		},
		{
			description: "replaces expired resources (within the override rotation window)",
			config: strings.TrimSpace(`
---
version: 1
namespace: team-name

stores:
- type: inprocess

requests:
- store: inprocess
  creds:
  - type: random
    rotation_window: 30m
    name: fake.state.id
			`),
			resources: []*sidecred.Resource{{
				Type:       sidecred.Randomized,
				ID:         testStateID,
				Store:      "inprocess",
				Expiration: time.Now().Add(29 * time.Minute),
			}},
			expectedResources: []*sidecred.Resource{{
				Type:       sidecred.Randomized,
				ID:         testStateID,
				Store:      "inprocess",
				Expiration: testTime,
				InUse:      true,
			}},
			expectedCreateCalls:  1,
			expectedDestroyCalls: 1,
		},
		{
			description: "does not replace resources (outside the rotation window)",
			config: strings.TrimSpace(`
---
version: 1
namespace: team-name

stores:
- type: inprocess

requests:
- store: inprocess
  creds:
  - type: random
    rotation_window: 240s
    name: fake.state.id
			`),
			resources: []*sidecred.Resource{{
				Type:       sidecred.Randomized,
				ID:         testStateID,
				Store:      "inprocess",
				Expiration: testTime.Add(-55 * time.Minute),
			}},
			expectedResources: []*sidecred.Resource{{
				Type:       sidecred.Randomized,
				ID:         testStateID,
				Store:      "inprocess",
				Expiration: testTime.Add(-55 * time.Minute),
				InUse:      true,
			}},
			expectedCreateCalls:  0,
			expectedDestroyCalls: 0,
		},
		{
			description: "destroys deposed resources",
			config: strings.TrimSpace(`
---
version: 1
namespace: team-name

stores:
- type: inprocess

requests:
- store: inprocess
  creds:
  - type: random
    name: fake.state.id
			`),
			resources: []*sidecred.Resource{{
				Type:       sidecred.Randomized,
				ID:         testStateID,
				Store:      "inprocess",
				Expiration: time.Now(),
			}},
			expectedResources: []*sidecred.Resource{{
				Type:       sidecred.Randomized,
				ID:         testStateID,
				Store:      "inprocess",
				Expiration: testTime,
				InUse:      true,
			}},
			expectedCreateCalls:  1,
			expectedDestroyCalls: 1,
		},
		{
			description: "destroys resources that are no longer requested",
			config: strings.TrimSpace(`
---
version: 1
namespace: team-name

stores:
- type: inprocess
			`),
			resources: []*sidecred.Resource{{
				Type:       sidecred.Randomized,
				ID:         "other.state.id",
				Store:      "inprocess",
				Expiration: testTime,
			}},
			expectedResources:    []*sidecred.Resource{},
			expectedDestroyCalls: 1,
		},
		{
			description: "does nothing if there are no requests",
			config: strings.TrimSpace(`
---
version: 1
namespace: team-name

stores:
- type: inprocess
			`),
			expectedSecrets: map[string]string{},
		},
		{
			description: "does nothing if there are no providers for the request",
			config: strings.TrimSpace(`
---
version: 1
namespace: team-name

stores:
- type: inprocess

requests:
- store: inprocess
  creds:
  - type: aws:sts
    name: fake.state.id
    config:
      role_arn: arn:aws:iam::role/role-name
			`),
			resources:         []*sidecred.Resource{},
			expectedSecrets:   map[string]string{},
			expectedResources: []*sidecred.Resource{},
		},
		{
			description: "allows different stores to have overlapping credential names",
			config: strings.TrimSpace(`
---
version: 1
namespace: team-name

stores:
- name: one
  type: inprocess
- name: two
  type: inprocess

requests:
- store: one
  creds:
  - type: random
    name: fake.state.id
- store: two
  creds:
  - type: random
    name: fake.state.id
			`),
			expectedResources: []*sidecred.Resource{
				{
					Type:       sidecred.Randomized,
					ID:         testStateID,
					Store:      "one",
					Expiration: testTime,
					InUse:      true,
				},
				{
					Type:       sidecred.Randomized,
					ID:         testStateID,
					Store:      "two",
					Expiration: testTime,
					InUse:      true,
				},
			},
			expectedCreateCalls: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			var (
				store    = inprocess.New()
				state    = sidecred.NewState()
				provider = &fakeProvider{}
			)
			for _, r := range tc.resources {
				state.AddResource(r)
			}

			s, err := sidecred.New([]sidecred.Provider{provider}, []sidecred.SecretStore{store}, 10*time.Minute)
			require.NoError(t, err)

			cfg, err := config.Parse([]byte(tc.config))
			require.NoError(t, err)

			err = s.Process(eventctx.TestContext(t), cfg, state)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedCreateCalls, provider.CreateCallCount(), "create calls")
			assert.Equal(t, tc.expectedDestroyCalls, provider.DestroyCallCount(), "destroy calls")

			for _, p := range state.Providers {
				assert.Equal(t, tc.expectedResources, p.Resources)
			}

			for k, v := range tc.expectedSecrets {
				value, found, err := store.Read(eventctx.TestContext(t), k, []byte("{}"))
				assert.NoError(t, err)
				assert.True(t, found, "secret exists")
				assert.Equal(t, v, value)
			}
		})
	}
}

// This test exists because looping over pointers as done when cleaning up expired/deposed
// resources (and deposed secrets) can lead to surprising behaviors. The test below ensures
// that things are working as intended.
func TestProcessCleanup(t *testing.T) {
	tests := []struct {
		description          string
		config               string
		resources            []*sidecred.Resource
		secrets              []*sidecred.Secret
		expectedDestroyCalls int
	}{
		{
			description: "cleanup works",
			config: strings.TrimSpace(`
---
version: 1
namespace: team-name

stores:
- type: inprocess
			`),
			resources: []*sidecred.Resource{
				{
					Type:       sidecred.Randomized,
					ID:         "r1",
					Expiration: time.Now(),
				},
				{
					Type:       sidecred.Randomized,
					ID:         "r2",
					Expiration: time.Now(),
				},
				{
					Type:       sidecred.Randomized,
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
			)

			for _, r := range tc.resources {
				state.AddResource(r)
			}

			for _, s := range tc.secrets {
				state.AddSecret(&sidecred.StoreConfig{Type: store.Type()}, s)
			}

			s, err := sidecred.New([]sidecred.Provider{provider}, []sidecred.SecretStore{store}, 10*time.Minute)
			require.NoError(t, err)

			cfg, err := config.Parse([]byte(tc.config))
			require.NoError(t, err)

			err = s.Process(eventctx.TestContext(t), cfg, state)
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

// Fake implementation of sidecred.Provider.
type fakeProvider struct {
	createCallCount  int
	destroyCallCount int
}

func (f *fakeProvider) Type() sidecred.ProviderType {
	return sidecred.Random
}

func (f *fakeProvider) Create(_ context.Context, _ *sidecred.CredentialRequest) ([]*sidecred.Credential, *sidecred.Metadata, error) {
	f.createCallCount++
	return []*sidecred.Credential{{
			Name:       "fake-credential",
			Value:      "fake-value",
			Expiration: testTime,
		}},
		nil,
		nil
}

func (f *fakeProvider) Destroy(_ context.Context, _ *sidecred.Resource) error {
	f.destroyCallCount++
	return nil
}

func (f *fakeProvider) CreateCallCount() int {
	return f.createCallCount
}

func (f *fakeProvider) DestroyCallCount() int {
	return f.destroyCallCount
}
