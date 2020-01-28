// Package s3 implements a sidecred.StateBackend using AWS S3.
package s3

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/telia-oss/sidecred"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// NewClient returns a new client for S3API.
func NewClient(sess *session.Session) S3API {
	return s3.New(sess)
}

// New returns a new sidecred.StateBackend for STS Credentials.
func New(client S3API, bucket, path string) sidecred.StateBackend {
	b := &backend{
		client: client,
		bucket: bucket,
		path:   path,
	}
	return b
}

type backend struct {
	client S3API
	bucket string
	path   string
}

// Load implements sidecred.StateBackend.
func (b *backend) Load() (*sidecred.State, error) {
	var state sidecred.State
	obj, err := b.client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(b.path),
	})
	if err != nil {
		e, ok := err.(awserr.Error)
		if !ok {
			return nil, err
		}
		if e.Code() == s3.ErrCodeNoSuchKey {
			return &state, nil
		}
		return nil, err
	}
	defer obj.Body.Close()
	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, obj.Body); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(buf.Bytes(), &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// Save implements sidecred.StateBackend.
func (b *backend) Save(state *sidecred.State) error {
	o, err := json.Marshal(state)
	if err != nil {
		return err
	}
	_, err = b.client.PutObject(&s3.PutObjectInput{
		Body:   aws.ReadSeekCloser(bytes.NewReader(o)),
		Bucket: aws.String(b.bucket),
		Key:    aws.String(b.path),
	})
	return err
}

// S3API wraps the interface for the API and provides a mocked implementation.
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . S3API
type S3API interface {
	GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error)
	PutObject(input *s3.PutObjectInput) (*s3.PutObjectOutput, error)
}
