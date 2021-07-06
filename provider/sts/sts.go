// Package sts implements a sidecred.Provider for AWS STS Credentials.
package sts

import (
	"fmt"
	"time"

	"github.com/telia-oss/sidecred"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
)

var _ sidecred.Validatable = &RequestConfig{}

// RequestConfig ...
type RequestConfig struct {
	RoleARN  string             `json:"role_arn"`
	Duration *sidecred.Duration `json:"duration"`
}

// Validate implements sidecred.Validatable.
func (c *RequestConfig) Validate() error {
	if c.RoleARN == "" {
		return fmt.Errorf("%q must be defined", "role_arn")
	}
	if c.Duration != nil && c.Duration.Seconds() < 900 {
		return fmt.Errorf("%q must be minimum 15min", "duration")
	}
	return nil
}

// NewClient returns a new client for STSAPI.
func NewClient(sess *session.Session) STSAPI {
	return sts.New(sess)
}

// New returns a new sidecred.Provider for STS Credentials.
func New(client STSAPI, options ...option) sidecred.Provider {
	p := &provider{
		client:          client,
		sessionDuration: 1 * time.Hour,
		externalID:      "",
	}
	for _, optionFunc := range options {
		optionFunc(p)
	}
	return p
}

type option func(*provider)

// WithSessionDuration overrides the default session duration.
func WithSessionDuration(duration time.Duration) option {
	return func(p *provider) {
		p.sessionDuration = duration
	}
}

// WithExternalID sets the external ID used to assume roles.
func WithExternalID(id string) option {
	return func(p *provider) {
		p.externalID = id
	}
}

type provider struct {
	client          STSAPI
	sessionDuration time.Duration
	externalID      string
}

// Type implements sidecred.Provider.
func (p *provider) Type() sidecred.ProviderType {
	return sidecred.AWS
}

// Create implements sidecred.Provider.
func (p *provider) Create(request *sidecred.CredentialRequest) ([]*sidecred.Credential, *sidecred.Metadata, error) {
	var c RequestConfig
	if err := request.UnmarshalConfig(&c); err != nil {
		return nil, nil, err
	}
	duration := int64(p.sessionDuration.Seconds())
	if c.Duration != nil {
		duration = int64(c.Duration.Seconds())
	}
	input := &sts.AssumeRoleInput{
		RoleSessionName: aws.String(request.Name),
		RoleArn:         aws.String(c.RoleARN),
		DurationSeconds: aws.Int64(duration),
	}
	if p.externalID != "" {
		input.SetExternalId(p.externalID)
	}
	output, err := p.client.AssumeRole(input)
	if err != nil {
		return nil, nil, fmt.Errorf("assume role: %s", err)
	}

	return []*sidecred.Credential{
		{
			Name:        request.Name + "-access-key",
			Value:       aws.StringValue(output.Credentials.AccessKeyId),
			Expiration:  aws.TimeValue(output.Credentials.Expiration),
			Description: "AWS credentials managed by sidecred.",
		},
		{
			Name:        request.Name + "-secret-key",
			Value:       aws.StringValue(output.Credentials.SecretAccessKey),
			Expiration:  aws.TimeValue(output.Credentials.Expiration),
			Description: "AWS credentials managed by sidecred.",
		},
		{
			Name:        request.Name + "-session-token",
			Value:       aws.StringValue(output.Credentials.SessionToken),
			Expiration:  aws.TimeValue(output.Credentials.Expiration),
			Description: "AWS credentials managed by sidecred.",
		},
	}, nil, nil
}

// Destroy implements sidecred.Provider.
func (p *provider) Destroy(_ *sidecred.Resource) error {
	return nil
}

// STSAPI wraps the interface for the API and provides a mocked implementation.
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . STSAPI
type STSAPI interface {
	AssumeRole(input *sts.AssumeRoleInput) (*sts.AssumeRoleOutput, error)
}
