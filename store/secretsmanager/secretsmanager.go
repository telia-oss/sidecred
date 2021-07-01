// Package secretsmanager implements sidecred.SecretStore on top of AWS Secrets Manager.
package secretsmanager

import (
	"encoding/json"
	"fmt"

	"github.com/telia-oss/sidecred"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
)

// NewClient returns a new SecretsManagerAPI client.
func NewClient(sess *session.Session) SecretsManagerAPI {
	return secretsmanager.New(sess)
}

// New creates a new sidecred.SecretStore using AWS Secrets Manager.
func New(client SecretsManagerAPI, options ...option) sidecred.SecretStore {
	s := &store{
		client:         client,
		secretTemplate: "/{{ .Namespace }}/{{ .Name }}",
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

type store struct {
	client         SecretsManagerAPI
	secretTemplate string
}

// config that can be passed to the Configure method of this store.
type config struct {
	SecretTemplate string `json:"secret_template"`
}

// Type implements sidecred.SecretStore.
func (s *store) Type() sidecred.StoreType {
	return sidecred.SecretsManager
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

	// Creating and handling the error results in fewer API calls than
	// checking if it exists before creating the secret and then updating it.
	_, err = s.client.CreateSecret(&secretsmanager.CreateSecretInput{
		Name:        aws.String(path),
		Description: aws.String(secret.Description),
	})
	if err != nil {
		e, ok := err.(awserr.Error)
		if !ok {
			return "", fmt.Errorf("convert aws error: %s", err)
		}
		if e.Code() != secretsmanager.ErrCodeResourceExistsException {
			return "", err
		}
	}

	_, err = s.client.UpdateSecret(&secretsmanager.UpdateSecretInput{
		SecretId:     aws.String(path),
		Description:  aws.String(secret.Description),
		SecretString: aws.String(secret.Value),
	})
	if err != nil {
		return "", err
	}

	return path, nil
}

// Read implements sidecred.SecretStore.
func (s *store) Read(path string, _ json.RawMessage) (string, bool, error) {
	out, err := s.client.GetSecretValue(&secretsmanager.GetSecretValueInput{
		SecretId: aws.String(path),
	})
	if err != nil {
		e, ok := err.(awserr.Error)
		if !ok {
			return "", false, fmt.Errorf("convert aws error: %v", err)
		}
		if e.Code() == secretsmanager.ErrCodeResourceNotFoundException {
			return "", false, nil
		}
		return "", false, err
	}
	// Ignoring SecretBinary since we'll only ever write to SecretString.
	return aws.StringValue(out.SecretString), true, nil
}

// Delete implements sidecred.SecretStore.
func (s *store) Delete(path string, _ json.RawMessage) error {
	_, err := s.client.DeleteSecret(&secretsmanager.DeleteSecretInput{
		SecretId:                   aws.String(path),
		ForceDeleteWithoutRecovery: aws.Bool(true),
	})
	if err != nil {
		e, ok := err.(awserr.Error)
		if !ok {
			return fmt.Errorf("convert aws error: %v", err)
		}
		if e.Code() == secretsmanager.ErrCodeResourceNotFoundException {
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

// SecretsManagerAPI wraps the interface for the API and provides a mocked implementation.
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . SecretsManagerAPI
type SecretsManagerAPI interface {
	CreateSecret(input *secretsmanager.CreateSecretInput) (*secretsmanager.CreateSecretOutput, error)
	UpdateSecret(input *secretsmanager.UpdateSecretInput) (*secretsmanager.UpdateSecretOutput, error)
	GetSecretValue(input *secretsmanager.GetSecretValueInput) (*secretsmanager.GetSecretValueOutput, error)
	DeleteSecret(input *secretsmanager.DeleteSecretInput) (*secretsmanager.DeleteSecretOutput, error)
}
