package sidecred

import (
	"fmt"
)

// Config represents the user-defined configuration that should be passed to the sidecred.Sidecred.Process method.
type Config struct {
	Version   int    `json:"version"`
	Namespace string `json:"namespace"`
	Stores    []struct {
		Type StoreType `json:"type"`
	}
	Requests []struct {
		Store string               `json:"store"`
		Creds []*CredentialRequest `json:"creds"`
	}
}

// Validate the configuration.
func (c *Config) Validate() error {
	if c.Version != 1 {
		return fmt.Errorf("invalid configuration version: %d", c.Version)
	}
	if c.Namespace == "" {
		return fmt.Errorf("%q must be defined", "namespace")
	}
	if len(c.Stores) == 0 {
		return fmt.Errorf("%q must be defined", "stores")
	}
	stores := make(map[string]struct{}, len(c.Stores))
	for i, s := range c.Stores {
		switch s.Type {
		case Inprocess, SSM, SecretsManager, GithubSecrets:
		default:
			return fmt.Errorf("stores[%d]: unknown type %q", i, string(s.Type))
		}
		if _, found := stores[string(s.Type)]; found {
			return fmt.Errorf("stores[%d]: duplicate store %q", i, string(s.Type))
		}
		stores[string(s.Type)] = struct{}{}
	}

	type requestsKey struct{ store, name string }
	requests := make(map[requestsKey]struct{}, len(c.Requests))

	for i, request := range c.Requests {
		if _, found := stores[request.Store]; !found {
			return fmt.Errorf("requests[%d]: undefined store %q", i, request.Store)
		}
		for ii, r := range request.Creds {
			switch r.Type {
			case AWSSTS, GithubAccessToken, GithubDeployKey, ArtifactoryAccessToken, Randomized:
			default:
				return fmt.Errorf("requests[%d]: creds[%d]: unknown type %q", i, ii, string(r.Type))
			}
			key := requestsKey{store: request.Store, name: r.Name}
			if _, found := requests[key]; found {
				return fmt.Errorf("requests[%d]: creds[%d]: duplicated request %+v", i, ii, key)
			}
			requests[key] = struct{}{}
		}
	}
	return nil
}
