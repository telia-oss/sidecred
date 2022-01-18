package main

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/telia-oss/sidecred/config"
)

// Verify that the testdata referenced in README.md is valid.
func TestUnmarshalTestData(t *testing.T) {
	b, err := ioutil.ReadFile("./testdata/config.yml")
	require.NoError(t, err)

	cfg, err := config.Parse(b)
	require.NoError(t, err)

	err = cfg.Validate()
	require.NoError(t, err)
}
