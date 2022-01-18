package sts_test

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/telia-oss/sidecred"
	provider "github.com/telia-oss/sidecred/provider/sts"
	"github.com/telia-oss/sidecred/provider/sts/stsfakes"
)

func TestSTSProvider(t *testing.T) {
	expectedCredentials := []*sidecred.Credential{
		{
			Name:        "request-name-access-key",
			Value:       "access-key",
			Description: "AWS credentials managed by sidecred.",
		},
		{
			Name:        "request-name-secret-key",
			Value:       "secret-key",
			Description: "AWS credentials managed by sidecred.",
		},
		{
			Name:        "request-name-session-token",
			Value:       "session-token",
			Description: "AWS credentials managed by sidecred.",
		},
	}
	tests := []struct {
		description             string
		externalID              string
		sessionDuration         time.Duration
		expectedSessionDuration int64
		request                 *sidecred.CredentialRequest
		stsAPIOutput            *sts.AssumeRoleOutput
	}{
		{
			description:             "sts provider works",
			externalID:              "externalID",
			sessionDuration:         30 * time.Minute,
			expectedSessionDuration: 1800,
			request: &sidecred.CredentialRequest{
				Type:   sidecred.AWSSTS,
				Name:   "request-name",
				Config: []byte(`{"role_arn": "request-role-arn"}`),
			},
		},
		{
			description:             "request duration overrides default",
			externalID:              "externalID",
			sessionDuration:         30 * time.Minute,
			expectedSessionDuration: 60,
			request: &sidecred.CredentialRequest{
				Type:   sidecred.AWSSTS,
				Name:   "request-name",
				Config: []byte(`{"role_arn": "request-role-arn", "duration":"60s"}`),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			fakeSTSAPI := &stsfakes.FakeSTSAPI{}
			fakeSTSAPI.AssumeRoleReturns(&sts.AssumeRoleOutput{
				Credentials: &sts.Credentials{
					AccessKeyId:     aws.String("access-key"),
					SecretAccessKey: aws.String("secret-key"),
					SessionToken:    aws.String("session-token"),
					Expiration:      aws.Time(time.Now().UTC()),
				},
			}, nil)

			p := provider.New(
				fakeSTSAPI,
				provider.WithSessionDuration(tc.sessionDuration),
				provider.WithExternalID(tc.externalID),
			)

			creds, metadata, err := p.Create(tc.request)
			require.NoError(t, err)
			require.Equal(t, 1, fakeSTSAPI.AssumeRoleCallCount())
			require.Len(t, creds, len(expectedCredentials))
			assert.Nil(t, metadata)

			for i, e := range expectedCredentials {
				assert.Equal(t, e.Name, creds[i].Name)
				assert.Equal(t, e.Value, creds[i].Value)
				assert.Equal(t, e.Description, creds[i].Description)
			}

			input := fakeSTSAPI.AssumeRoleArgsForCall(0)
			assert.Equal(t, tc.expectedSessionDuration, aws.Int64Value(input.DurationSeconds))
			assert.Equal(t, tc.externalID, aws.StringValue(input.ExternalId))
		})
	}
}
