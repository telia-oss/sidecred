package main

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/telia-oss/sidecred"
	"sigs.k8s.io/yaml"
)

// Verify that the testdata referenced in README.md is valid.
func TestUnmarshalTestData(t *testing.T) {
	b, err := ioutil.ReadFile("./testdata/config.yml")
	require.NoError(t, err)

	var config sidecred.Config
	err = yaml.UnmarshalStrict(b, &config)
	require.NoError(t, err)

	err = config.Validate()
	require.NoError(t, err)
}
