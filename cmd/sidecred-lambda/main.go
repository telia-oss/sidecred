package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"

	"github.com/telia-oss/sidecred"
	"github.com/telia-oss/sidecred/internal/cli"

	"github.com/alecthomas/kingpin"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

var version string

func main() {
	var (
		app    = kingpin.New("sidecred", "Sideload your credentials.").Version(version).Writer(os.Stdout).DefaultEnvars()
		bucket = app.Flag("config-bucket", "Name of the S3 bucket where the config is stored.").Required().String()
	)
	cli.Setup(app, runFunc(bucket), nil, nil)
	kingpin.MustParse(app.Parse(os.Args[1:]))
}

// Event is the expected payload sent to the Lambda.
type Event struct {
	Namespace  string `json:"namespace"`
	ConfigPath string `json:"path"`
}

func runFunc(configBucket *string) func(s *sidecred.Sidecred) error {
	return func(s *sidecred.Sidecred) error {
		lambda.Start(func(event Event) error {
			requests, err := loadConfig(*configBucket, event.ConfigPath)
			if err != nil {
				return err
			}
			return s.Process(event.Namespace, requests)
		})
		return nil
	}
}

func loadConfig(bucket, key string) ([]*sidecred.Request, error) {
	sess, err := session.NewSession()
	if err != nil {
		return nil, err
	}
	client := s3.New(sess)

	var requests []*sidecred.Request
	obj, err := client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer obj.Body.Close()
	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, obj.Body); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(buf.Bytes(), &requests); err != nil {
		return nil, err
	}
	return requests, nil
}
