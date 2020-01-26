// Package ssm implements sidecred.SecretStore on top of AWS Parameter store.
package ssm

import (
	"fmt"

	"github.com/telia-oss/sidecred"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// NewClient returns a new SSMIAPI client.
func NewClient(sess *session.Session) SSMAPI {
	return ssm.New(sess)
}

// New creates a new sidecred.SecretStore using AWS Secrets Manager.
func New(client SSMAPI, options ...option) sidecred.SecretStore {
	s := &store{
		client:       client,
		pathTemplate: "/{{ .Namespace }}/{{ .Name }}",
		kmsKeyID:     "",
	}
	for _, optionFunc := range options {
		optionFunc(s)
	}
	return s
}

type option func(*store)

// WithPathTemplate sets the path template when instanciating a new store.
func WithPathTemplate(t string) option {
	return func(s *store) {
		s.pathTemplate = t
	}
}

// WithKMSKeyID sets the default KMS key ID to use when encrypting the secret.
func WithKMSKeyID(k string) option {
	return func(s *store) {
		s.kmsKeyID = k
	}
}

type store struct {
	client       SSMAPI
	pathTemplate string
	kmsKeyID     string
}

// Type implements sidecred.SecretStore.
func (s *store) Type() sidecred.StoreType {
	return sidecred.SSM
}

// Write implements sidecred.SecretStore.
func (s *store) Write(namespace string, secret *sidecred.Credential) (string, error) {
	path, err := sidecred.BuildSecretPath(s.pathTemplate, namespace, secret.Name)
	if err != nil {
		return "", fmt.Errorf("build secret path: %s", err)
	}

	input := &ssm.PutParameterInput{
		Name:        aws.String(path),
		Description: aws.String(secret.Description),
		Value:       aws.String(secret.Value),
		Type:        aws.String("SecureString"),
		Overwrite:   aws.Bool(true),
	}

	if s.kmsKeyID != "" {
		input.SetKeyId(s.kmsKeyID)
	}

	if _, err = s.client.PutParameter(input); err != nil {
		return "", err
	}
	return path, nil
}

// Read implements sidecred.SecretStore.
func (s *store) Read(path string) (string, bool, error) {
	out, err := s.client.GetParameter(&ssm.GetParameterInput{
		Name:           aws.String(path),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		e, ok := err.(awserr.Error)
		if !ok {
			return "", false, fmt.Errorf("convert aws error: %v", err)
		}
		if e.Code() == ssm.ErrCodeParameterNotFound {
			return "", false, nil
		}
		return "", false, err
	}

	return aws.StringValue(out.Parameter.Value), true, nil
}

// Delete implements sidecred.SecretStore.
func (s *store) Delete(path string) error {
	_, err := s.client.DeleteParameter(&ssm.DeleteParameterInput{
		Name: aws.String(path),
	})
	if err != nil {
		e, ok := err.(awserr.Error)
		if !ok {
			return fmt.Errorf("convert aws error: %v", err)
		}
		if e.Code() == ssm.ErrCodeParameterNotFound {
			return nil
		}
		return err
	}
	return nil
}

// SSMAPI wraps the interface for the API and provides a mocked implementation.
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . SSMAPI
type SSMAPI interface {
	PutParameter(input *ssm.PutParameterInput) (*ssm.PutParameterOutput, error)
	GetParameter(input *ssm.GetParameterInput) (*ssm.GetParameterOutput, error)
	DeleteParameter(input *ssm.DeleteParameterInput) (*ssm.DeleteParameterOutput, error)
}
