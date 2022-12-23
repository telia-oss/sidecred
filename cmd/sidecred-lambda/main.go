package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"os"
	"time"

	"github.com/alecthomas/kingpin"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	environment "github.com/telia-oss/aws-env"
	"go.uber.org/zap"

	"github.com/telia-oss/sidecred"
	"github.com/telia-oss/sidecred/config"
	"github.com/telia-oss/sidecred/eventctx"
	"github.com/telia-oss/sidecred/internal/cli"
)

var version string

func main() {
	var (
		app    = kingpin.New("sidecred", "Sideload your credentials.").Version(version).UsageWriter(os.Stdout).ErrorWriter(os.Stdout).DefaultEnvars()
		bucket = app.Flag("config-bucket", "Name of the S3 bucket where the config is stored.").Required().String()
	)

	// Make span, and trace id random
	rand.Seed(time.Now().Unix())

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

func runFunc(configBucket *string) func(*sidecred.Sidecred, sidecred.StateBackend, sidecred.RunConfig) error {
	return func(s *sidecred.Sidecred, backend sidecred.StateBackend, runConfig sidecred.RunConfig) error {
		lambda.Start(func(event Event) error {
			ctx := context.Background() // NOTE: change to function arg later.

			uid := rand.Uint64() //nolint:gosec // Only need random enough for unique id
			ctx = eventctx.SetLogger(ctx, runConfig.Logger.With(
				zap.Uint64("dd.trace_id", uid),
				zap.Uint64("dd.span_id", uid),
			))

			ctx = eventctx.SetStats(ctx, &eventctx.Stats{
				CallsToGithub: 0,
			})

			cfg, err := loadConfig(ctx, *configBucket, event.ConfigPath)
			if err != nil {
				return failure(ctx, cfg.Namespace(), fmt.Errorf("failed to load config: %s", err))
			}

			ctx = eventctx.SetLogger(ctx, eventctx.GetLogger(ctx).With(
				zap.String("namespace", cfg.Namespace()),
			))

			state, err := backend.Load(ctx, event.StatePath)
			if err != nil {
				return failure(ctx, cfg.Namespace(), fmt.Errorf("failed to load state: %s", err))
			}

			if err := s.Process(ctx, cfg, state); err != nil {
				return failure(ctx, cfg.Namespace(), err)
			}

			if err := backend.Save(ctx, event.StatePath, state); err != nil {
				return failure(ctx, cfg.Namespace(), fmt.Errorf("failed to save state: %s", err))
			}

			stats := eventctx.GetStats(ctx)
			eventctx.GetLogger(ctx).Info(fmt.Sprintf("processing '%s' done", cfg.Namespace()),
				zap.Int("calls_to_github", stats.CallsToGithub),
			)

			return nil
		})
		return nil
	}
}

func failure(ctx context.Context, namespace string, err error) error {
	stats := eventctx.GetStats(ctx)
	eventctx.GetLogger(ctx).Info(fmt.Sprintf("processing '%s' failed", namespace),
		zap.Int("calls_to_github", stats.CallsToGithub),
	)

	return err
}

func loadConfig(ctx context.Context, bucket, key string) (sidecred.Config, error) {
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
