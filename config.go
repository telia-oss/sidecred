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
	Requests []*RequestConfig `json:"requests"`
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
		for ii, cred := range request.Creds {
			if err := cred.validate(); err != nil {
				return fmt.Errorf("requests[%d]: creds[%d]: %s", i, ii, err)
			}
			for _, r := range cred.flatten() {
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
	}
	return nil
}

// RequestConfig maps credential requests to a secret store, and is part of the configuration format for Sidecred.
type RequestConfig struct {
	Store string                     `json:"store"`
	Creds []*CredentialRequestConfig `json:"creds"`
}

// CredentialRequests returns the flattened list of CredentialRequest's.
func (r *RequestConfig) CredentialRequests() (requests []*CredentialRequest) {
	for _, cred := range r.Creds {
		requests = append(requests, cred.flatten()...)
	}
	return requests
}

// CredentialRequestConfig extends sidecred.CredentialRequest by allowing it to be defined in two ways:
// 1. As a regular CredentialRequest.
// 2. As a list of requests that share a CredentialType (nested credential requests should omit "type"):
//
//  - type: aws:sts
//    list:
// 	    - name: credential1
//        config ...
// 	    - name: credential2
//        config ...
//
type CredentialRequestConfig struct {
	*CredentialRequest `json:",inline"`
	List               []*CredentialRequest `json:"list,omitempty"`
}

// validate the configRequest.
func (c *CredentialRequestConfig) validate() error {
	if len(c.List) == 0 {
		return nil // config.Validate covers the inlined request.
	}
	if c.CredentialRequest.Name != "" {
		return fmt.Errorf("%q should not be specified for lists", "name")
	}
	if len(c.CredentialRequest.Config) > 0 {
		return fmt.Errorf("%q should not be specified for lists", "config")
	}
	for i, r := range c.List {
		if r.Type != "" {
			return fmt.Errorf("list entry[%d]: request should not include %q", i, "type")
		}
	}
	return nil
}

// flatten returns the flattened list of credential requests.
func (c *CredentialRequestConfig) flatten() []*CredentialRequest {
	if len(c.List) == 0 {
		return []*CredentialRequest{c.CredentialRequest}
	}
	var requests []*CredentialRequest
	for _, r := range c.List {
		r.Type = c.CredentialRequest.Type
		requests = append(requests, r)
	}
	return c.List
}
