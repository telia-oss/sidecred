package sidecred

import (
	"fmt"
)

// Config represents the user-defined configuration that should be passed to the sidecred.Sidecred.Process method.
type Config struct {
	Version   int        `json:"version"`
	Namespace string     `json:"namespace"`
	Requests  []*Request `json:"requests"`
}

// Validate the configuration.
func (c *Config) Validate() error {
	if c.Version != 1 {
		return fmt.Errorf("invalid configuration version: %d", c.Version)
	}
	if c.Namespace == "" {
		return fmt.Errorf("%q must be defined", "namespace")
	}
	requests := make(map[string]struct{}, len(c.Requests))
	for i, r := range c.Requests {
		switch r.Type {
		case AWSSTS, GithubAccessToken, GithubDeployKey, ArtifactoryAccessToken, Randomized:
		default:
			return fmt.Errorf("requests[%d]: unknown type %q", i, string(r.Type))
		}
		if _, found := requests[r.Name]; found {
			return fmt.Errorf("requests[%d]: duplicate request %q", i, r.Name)
		}
		requests[r.Name] = struct{}{}
	}
	return nil
}
