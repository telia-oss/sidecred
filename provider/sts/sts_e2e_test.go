// +build e2e

package sts_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/telia-oss/sidecred"
	provider "github.com/telia-oss/sidecred/provider/sts"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/stretchr/testify/require"
)

func TestSTSProviderE2E(t *testing.T) {
	tests := []struct {
		description string
	}{
		{
			description: "sts provider works",
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			sess, err := session.NewSession(&aws.Config{Region: aws.String("eu-west-1")})
			require.NoError(t, err)

			// Create a temporary role with zero permissions.
			roleName, roleARN := createIAMRole(t, sess)
			defer deleteIAMRole(t, sess, roleName)

			// Sleep so STS has time to learn about the new role (did not work with <10 sec sleep).
			time.Sleep(10 * time.Second)

			p := provider.New(
				sts.New(sess),
				provider.WithSessionDuration(15*time.Minute),
				provider.WithExternalID("sidecred-e2e-test"),
			)

			_, _, err = p.Create(&sidecred.CredentialRequest{
				Type:   sidecred.AWSSTS,
				Name:   "request-name",
				Config: []byte(fmt.Sprintf(`{"role_arn":"%s"}`, roleARN)),
			})
			require.NoError(t, err)
		})
	}
}

func generateTrustPolicy(t *testing.T, sess *session.Session) string {
	c := sts.New(sess)

	out, err := c.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		t.Fatalf("get caller identity: %s", err)
	}

	policy := fmt.Sprintf(strings.TrimSpace(`
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "",
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::%s:root"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
	`), aws.StringValue(out.Account))
	return policy
}

func createIAMRole(t *testing.T, sess *session.Session) (string, string) {
	c := iam.New(sess)

	out, err := c.CreateRole(&iam.CreateRoleInput{
		RoleName:                 aws.String("sidecred-e2e-test-role"),
		Description:              aws.String("sidecred e2e test role"),
		MaxSessionDuration:       aws.Int64(3600),
		AssumeRolePolicyDocument: aws.String(generateTrustPolicy(t, sess)),
	})
	if err != nil {
		e, ok := err.(awserr.Error)
		if !ok || e.Code() != iam.ErrCodeEntityAlreadyExistsException {
			t.Fatalf("create role: %s", err)
		}
	}

	return aws.StringValue(out.Role.RoleName), aws.StringValue(out.Role.Arn)
}

func deleteIAMRole(t *testing.T, sess *session.Session, roleName string) {
	c := iam.New(sess)

	_, err := c.DeleteRole(&iam.DeleteRoleInput{RoleName: aws.String(roleName)})
	if err != nil {
		e, ok := err.(awserr.Error)
		if !ok || e.Code() != iam.ErrCodeNoSuchEntityException {
			t.Fatalf("delete role: %s", err)
		}
	}
}
