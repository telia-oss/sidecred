package s3_test

import (
	"io/ioutil"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/telia-oss/sidecred"
	backend "github.com/telia-oss/sidecred/backend/s3"
	"github.com/telia-oss/sidecred/backend/s3/s3fakes"
)

func TestS3Backend(t *testing.T) {
	tests := []struct {
		description string
	}{
		{
			description: "s3 backend works",
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			fakeS3 := &s3fakes.FakeS3API{}
			fakeS3.GetObjectReturns(&s3.GetObjectOutput{Body: ioutil.NopCloser(strings.NewReader("{}"))}, nil)

			b := backend.New(fakeS3, "bucket")
			state, err := b.Load("key")
			require.NoError(t, err)
			assert.Equal(t, &sidecred.State{}, state)
		})
	}
}
