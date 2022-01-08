package main

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/telia-oss/sidecred"
	"github.com/telia-oss/sidecred/config"
	"github.com/telia-oss/sidecred/internal/cli"

	"github.com/alecthomas/kingpin"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	environment "github.com/telia-oss/aws-env"
)

var version string

func main() {
	var (
		app    = kingpin.New("sidecred", "Sideload your credentials.").Version(version).UsageWriter(os.Stdout).ErrorWriter(os.Stdout).DefaultEnvars()
		bucket = app.Flag("config-bucket", "Name of the S3 bucket where the config is stored.").Required().String()
	)

	sess, err := session.NewSession()
	if err != nil {
		panic(fmt.Errorf("failed to create a new session: %s", err))
	}

	// Exchange secrets in environment variables with their values.
	env, err := environment.New(sess)
	if err != nil {
		panic(fmt.Errorf("failed to initialize aws-env: %s", err))
	}

	if err := env.Populate(); err != nil {
		panic(fmt.Errorf("failed to populate environment: %s", err))
	}

	cli.AddRunCommand(app, runFunc(bucket), nil, nil).Default()
	kingpin.MustParse(app.Parse(os.Args[1:]))
}

// Event is the expected payload sent to the Lambda.
type Event struct {
	ConfigPath string `json:"config_path"`
	StatePath  string `json:"state_path"`
}

func runFunc(configBucket *string) func(*sidecred.Sidecred, sidecred.StateBackend) error {
	return func(s *sidecred.Sidecred, backend sidecred.StateBackend) error {
		lambda.Start(func(event Event) error {
			cfg, err := loadConfig(*configBucket, event.ConfigPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %s", err)
			}
			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("failed to validate config: %s", err)
			}
			state, err := backend.Load(event.StatePath)
			if err != nil {
				return fmt.Errorf("failed to load state: %s", err)
			}
			defer func(backend sidecred.StateBackend, path string, state *sidecred.State) {
				err := backend.Save(path, state)
				if err != nil {
					fmt.Printf("backend save error on path %s: error %s", path, err)
				}
			}(backend, event.StatePath, state)
			return s.Process(cfg, state)
		})
		return nil
	}
}

func loadConfig(bucket, key string) (sidecred.Config, error) {
	sess, err := session.NewSession()
	if err != nil {
		return nil, err
	}
	client := s3.New(sess)

	obj, err := client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer obj.Body.Close()

	b := bytes.NewBuffer(nil)
	if _, err := io.Copy(b, obj.Body); err != nil {
		return nil, err
	}

	return config.Parse(b.Bytes())
}
