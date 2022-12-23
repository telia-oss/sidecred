// Package s3 implements a sidecred.StateBackend using AWS S3.
package s3

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/telia-oss/sidecred"
)

// NewClient returns a new client for S3API.
func NewClient(sess *session.Session) S3API {
	return s3.New(sess)
}

// New returns a new sidecred.StateBackend for STS Credentials.
func New(client S3API, bucket string) sidecred.StateBackend {
	b := &backend{
		client: client,
		bucket: bucket,
	}
	return b
}

type backend struct {
	client S3API
	bucket string
}

// Load implements sidecred.StateBackend.
func (b *backend) Load(ctx context.Context, key string) (*sidecred.State, error) {
	var state sidecred.State
	obj, err := b.client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var e awserr.Error
		if !errors.As(err, &e) {
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
func (b *backend) Save(ctx context.Context, key string, state *sidecred.State) error {
	o, err := json.Marshal(state)
	if err != nil {
		return err
	}
	_, err = b.client.PutObject(&s3.PutObjectInput{
		Body:   aws.ReadSeekCloser(bytes.NewReader(o)),
		Bucket: aws.String(b.bucket),
		Key:    aws.String(key),
	})
	return err
}

// S3API wraps the interface for the API and provides a mocked implementation.
//counterfeiter:generate . S3API
type S3API interface {
	GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error)
	PutObject(input *s3.PutObjectInput) (*s3.PutObjectOutput, error)
}
