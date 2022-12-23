package cli_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/alecthomas/kingpin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"

	"github.com/telia-oss/sidecred"
	"github.com/telia-oss/sidecred/backend/s3"
	"github.com/telia-oss/sidecred/backend/s3/s3fakes"
	"github.com/telia-oss/sidecred/config"
	"github.com/telia-oss/sidecred/eventctx"
	"github.com/telia-oss/sidecred/internal/cli"
	"github.com/telia-oss/sidecred/provider/sts"
	"github.com/telia-oss/sidecred/provider/sts/stsfakes"
	"github.com/telia-oss/sidecred/store/secretsmanager"
	"github.com/telia-oss/sidecred/store/secretsmanager/secretsmanagerfakes"
	"github.com/telia-oss/sidecred/store/ssm"
	"github.com/telia-oss/sidecred/store/ssm/ssmfakes"
)

func testAWSClientFactory() (s3.S3API, sts.STSAPI, ssm.SSMAPI, secretsmanager.SecretsManagerAPI) {
	return &s3fakes.FakeS3API{}, &stsfakes.FakeSTSAPI{}, &ssmfakes.FakeSSMAPI{}, &secretsmanagerfakes.FakeSecretsManagerAPI{}
}

func TestCLI(t *testing.T) {
	tests := []struct {
		description string
		command     []string
		expected    string
	}{
		{
			description: "works",
			command:     []string{"--state-backend", "file", "--debug"},
			expected: strings.TrimSpace(`
{"level":"info","msg":"starting sidecred","namespace":"example","requests":1}
{"level":"info","msg":"processing request","namespace":"example","type":"random","store":"inprocess","name":"example-random-credential"}
{"level":"info","msg":"created new credentials","namespace":"example","type":"random","store":"inprocess","count":1}
{"level":"debug","msg":"start creds for-loop","namespace":"example","type":"random","store":"inprocess"}
{"level":"debug","msg":"wrote to store","namespace":"example","type":"random","store":"inprocess","name":"example-random-credential"}
{"level":"debug","msg":"stored credential","namespace":"example","type":"random","store":"inprocess","path":"example.example-random-credential"}
{"level":"info","msg":"done processing","namespace":"example","type":"random","store":"inprocess"}
             `),
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			b := &zaptest.Buffer{}
			loggerFactory := func(bool) (*zap.Logger, error) {
				c := zap.NewProductionEncoderConfig()
				c.TimeKey = ""
				e := zapcore.NewJSONEncoder(c)
				l := zap.New(zapcore.NewCore(e, zapcore.AddSync(b), zapcore.DebugLevel))
				return l, nil
			}

			cfg := strings.TrimSpace(`
---
version: 1
namespace: example

stores:
  - type: inprocess

requests:
  - store: inprocess
    creds:
    - type: random
      name: example-random-credential
      config:
        length: 10
            `)

			runFunc := func(s *sidecred.Sidecred, _ sidecred.StateBackend, runConfig sidecred.RunConfig) error {
				c, err := config.Parse([]byte(cfg))
				if err != nil {
					return fmt.Errorf("failed to parse config: %s", err)
				}

				ctx := eventctx.SetLogger(context.TODO(), runConfig.Logger.With(
					zap.String("namespace", "example"),
				))

				return s.Process(ctx, c, &sidecred.State{})
			}

			app := kingpin.New("test", "").Terminate(nil)
			cli.AddRunCommand(app, runFunc, testAWSClientFactory, loggerFactory).Default()

			_, err := app.Parse(tc.command)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, strings.TrimSpace(b.String()))
		})
	}
}
