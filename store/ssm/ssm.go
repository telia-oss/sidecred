// Package ssm implements sidecred.SecretStore on top of AWS Parameter store.
package ssm

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"

	"github.com/telia-oss/sidecred"
)

// NewClient returns a new SSMAPI client.
func NewClient(sess *session.Session) SSMAPI {
	return ssm.New(sess)
}

// New creates a new sidecred.SecretStore using AWS Secrets Manager.
func New(client SSMAPI, options ...option) sidecred.SecretStore {
	s := &store{
		client:         client,
		secretTemplate: "/{{ .Namespace }}/{{ .Name }}",
		kmsKeyID:       "",
	}
	for _, optionFunc := range options {
		optionFunc(s)
	}
	return s
}

type option func(*store)

// WithSecretTemplate sets the path template when instantiating a new store.
func WithSecretTemplate(t string) option {
	return func(s *store) {
		s.secretTemplate = t
	}
}

// WithKMSKeyID sets the default KMS key ID to use when encrypting the secret.
func WithKMSKeyID(k string) option {
	return func(s *store) {
		s.kmsKeyID = k
	}
}

type store struct {
	client         SSMAPI
	secretTemplate string
	kmsKeyID       string
}

// config that can be passed to the Configure method of this store.
type config struct {
	SecretTemplate string `json:"secret_template"`
}

// Type implements sidecred.SecretStore.
func (s *store) Type() sidecred.StoreType {
	return sidecred.SSM
}

// Write implements sidecred.SecretStore.
func (s *store) Write(namespace string, secret *sidecred.Credential, config json.RawMessage) (string, error) {
	c, err := s.parseConfig(config)
	if err != nil {
		return "", fmt.Errorf("parse config: %s", err)
	}
	path, err := sidecred.BuildSecretTemplate(c.SecretTemplate, namespace, secret.Name)
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
func (s *store) Read(path string, _ json.RawMessage) (string, bool, error) {
	out, err := s.client.GetParameter(&ssm.GetParameterInput{
		Name:           aws.String(path),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		var e awserr.Error
		if !errors.As(err, &e) {
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
func (s *store) Delete(path string, _ json.RawMessage) error {
	_, err := s.client.DeleteParameter(&ssm.DeleteParameterInput{
		Name: aws.String(path),
	})
	if err != nil {
		var e awserr.Error
		if !errors.As(err, &e) {
			return fmt.Errorf("convert aws error: %v", err)
		}
		if e.Code() == ssm.ErrCodeParameterNotFound {
			return nil
		}
		return err
	}
	return nil
}

// parseConfig parses and validates the config.
func (s *store) parseConfig(raw json.RawMessage) (*config, error) {
	c := &config{}
	if err := sidecred.UnmarshalConfig(raw, &c); err != nil {
		return nil, err
	}
	if c.SecretTemplate == "" {
		c.SecretTemplate = s.secretTemplate
	}
	return c, nil
}

// SSMAPI wraps the interface for the API and provides a mocked implementation.
//counterfeiter:generate . SSMAPI
type SSMAPI interface {
	PutParameter(input *ssm.PutParameterInput) (*ssm.PutParameterOutput, error)
	GetParameter(input *ssm.GetParameterInput) (*ssm.GetParameterOutput, error)
	DeleteParameter(input *ssm.DeleteParameterInput) (*ssm.DeleteParameterOutput, error)
}
