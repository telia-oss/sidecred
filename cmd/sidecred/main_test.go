package main

import (
	"io/ioutil"
	"testing"

	"github.com/telia-oss/sidecred"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestUnmarshalConfig(t *testing.T) {
	input, err := ioutil.ReadFile("./testdata/config.yml")
	require.NoError(t, err)

	var requests []*sidecred.Request
	err = yaml.Unmarshal(input, &requests)
	require.NoError(t, err)

	expected := []sidecred.CredentialType{sidecred.AWSSTS, sidecred.GithubAccessToken, sidecred.GithubDeployKey}
	require.Equal(t, len(expected), len(requests))
	for i, e := range expected {
		assert.Equal(t, e, requests[i].Type)
	}
}
