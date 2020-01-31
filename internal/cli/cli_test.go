package cli_test

import (
	"strings"
	"testing"

	"github.com/telia-oss/sidecred"
	"github.com/telia-oss/sidecred/backend/s3"
	"github.com/telia-oss/sidecred/backend/s3/s3fakes"
	"github.com/telia-oss/sidecred/internal/cli"
	"github.com/telia-oss/sidecred/provider/sts"
	"github.com/telia-oss/sidecred/provider/sts/stsfakes"
	"github.com/telia-oss/sidecred/store/secretsmanager"
	"github.com/telia-oss/sidecred/store/secretsmanager/secretsmanagerfakes"
	"github.com/telia-oss/sidecred/store/ssm"
	"github.com/telia-oss/sidecred/store/ssm/ssmfakes"

	"github.com/alecthomas/kingpin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
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
			command:     []string{"--state-backend", "file", "--secret-store-backend", "inprocess", "--debug"},
			expected: strings.TrimSpace(`
{"level":"info","msg":"starting sidecred","namespace":"example","requests":1}
{"level":"info","msg":"processing request","namespace":"example","type":"random","name":"example-random-credential"}
{"level":"info","msg":"created new credentials","namespace":"example","type":"random","count":1}
{"level":"debug","msg":"stored credential","namespace":"example","type":"random","path":"example.example-random-credential"}
{"level":"info","msg":"done processing","namespace":"example","type":"random"}
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

			runFunc := func(s *sidecred.Sidecred, _ sidecred.StateBackend) error {
				return s.Process("example", []*sidecred.Request{{
					Type:   sidecred.Randomized,
					Name:   "example-random-credential",
					Config: []byte(`{"length":10}`),
				}}, &sidecred.State{})
			}

			app := kingpin.New("test", "").Terminate(nil)
			cli.Setup(app, runFunc, testAWSClientFactory, loggerFactory)

			_, err := app.Parse(tc.command)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, strings.TrimSpace(b.String()))
		})
	}
}
