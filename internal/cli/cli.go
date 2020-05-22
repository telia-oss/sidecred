package cli

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/telia-oss/sidecred"
	"github.com/telia-oss/sidecred/backend/file"
	"github.com/telia-oss/sidecred/backend/s3"
	"github.com/telia-oss/sidecred/provider/artifactory"
	"github.com/telia-oss/sidecred/provider/github"
	"github.com/telia-oss/sidecred/provider/random"
	"github.com/telia-oss/sidecred/provider/sts"
	"github.com/telia-oss/sidecred/store/inprocess"
	"github.com/telia-oss/sidecred/store/secretsmanager"
	"github.com/telia-oss/sidecred/store/ssm"

	"github.com/alecthomas/kingpin"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/telia-oss/githubapp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Type definitions that allow us to reuse the CLI (flags and setup) between binaries, and
// also so we can pass in test fakes during testing.
type (
	runFunc          func(*sidecred.Sidecred, sidecred.StateBackend) error
	awsClientFactory func() (s3.S3API, sts.STSAPI, ssm.SSMAPI, secretsmanager.SecretsManagerAPI)
	loggerFactory    func(bool) (*zap.Logger, error)
)

// Setup a kingpin.Application to run sidecred.
func Setup(app *kingpin.Application, run runFunc, newAWSClient awsClientFactory, newLogger loggerFactory) {
	var (
		randomProviderRotationInterval     = app.Flag("random-provider-rotation-interval", "Rotation interval for the random provider").Default("168h").Duration()
		stsProviderEnabled                 = app.Flag("sts-provider-enabled", "Enable the STS provider").Bool()
		stsProviderExternalID              = app.Flag("sts-provider-external-id", "External ID for the STS Provider").String()
		stsProviderSessionDuration         = app.Flag("sts-provider-session-duration", "Session duration for STS credentials").Default("1h").Duration()
		githubProviderEnabled              = app.Flag("github-provider-enabled", "Enable the Github provider").Bool()
		githubProviderIntegrationID        = app.Flag("github-provider-integration-id", "Github Apps integration ID").Int64()
		githubProviderPrivateKey           = app.Flag("github-provider-private-key", "Github apps private key").String()
		githubProviderKeyRotationInterval  = app.Flag("github-provider-key-rotation-interval", "Rotation interval for deploy keys").Default("168h").Duration()
		artifactoryProviderEnabled         = app.Flag("artifactory-provider-enabled", "Enable the Artifactory provider").Bool()
		artifactoryProviderHostname        = app.Flag("artifactory-provider-hostname", "Hostname for the Artifactory Provider").String()
		artifactoryProviderUsername        = app.Flag("artifactory-provider-username", "Username for the Artifactory Provider").String()
		artifactoryProviderPassword        = app.Flag("artifactory-provider-password", "Password for the Artifactory Provider").String()
		artifactoryProviderAccessToken     = app.Flag("artifactory-provider-access-token", "Access token for the Artifactory Provider").String()
		artifactoryProviderAPIKey          = app.Flag("artifactory-provider-api-key", "API key for the Artifactory Provider").String()
		artifactoryProviderSessionDuration = app.Flag("artifactory-provider-session-duration", "Session duration for artifactory tokens").Default("1h").Duration()
		secretStoreBackend                 = app.Flag("secret-store-backend", "Backend to use for secrets").Required().String()
		inprocessStorePathTemplate         = app.Flag("inprocess-store-path-template", "Path template to use for the inprocess store").Default("{{ .Namespace }}.{{ .Name }}").String()
		secretsManagerStorePathTemplate    = app.Flag("secrets-manager-store-path-template", "Path template to use for the secrets manager store").Default("/{{ .Namespace }}/{{ .Name }}").String()
		ssmStorePathTemplate               = app.Flag("ssm-store-path-template", "Path template to use for SSM Parameter store").Default("/{{ .Namespace }}/{{ .Name }}").String()
		ssmStoreKMSKeyID                   = app.Flag("ssm-store-kms-key-id", "KMS key to use for encrypting secrets stored in SSM Parameter store").String()
		stateBackend                       = app.Flag("state-backend", "Backend to use for storing state").Required().String()
		s3BackendBucket                    = app.Flag("s3-backend-bucket", "Bucket name to use for the S3 state backend").String()
		rotationWindow                     = app.Flag("rotation-window", "A window in time (duration) where sidecred should rotate credentials prior to their expiration").Default("10m").Duration()
		debug                              = app.Flag("debug", "Enable debug logging").Bool()
	)

	app.Action(func(_ *kingpin.ParseContext) error {
		if newLogger == nil {
			newLogger = defaultLogger
		}
		if newAWSClient == nil {
			newAWSClient = defaultAWSClientFactory
		}
		logger, err := newLogger(*debug)
		if err != nil {
			panic(fmt.Errorf("initialize zap logger: %s", err))
		}
		defer logger.Sync()

		providers := []sidecred.Provider{random.New(
			time.Now().UnixNano(),
			random.WithRotationInterval(*randomProviderRotationInterval),
		)}

		if *stsProviderEnabled {
			_, client, _, _ := newAWSClient()
			providers = append(providers, sts.New(client,
				sts.WithExternalID(*stsProviderExternalID),
				sts.WithSessionDuration(*stsProviderSessionDuration),
			))
		}

		if *githubProviderEnabled {
			client, err := githubapp.NewClient(*githubProviderIntegrationID, []byte(*githubProviderPrivateKey))
			if err != nil {
				logger.Fatal("initialize github app", zap.Error(err))
			}
			providers = append(providers, github.New(
				githubapp.New(client),
				github.WithDeployKeyRotationInterval(*githubProviderKeyRotationInterval),
			))
		}

		if *artifactoryProviderEnabled {
			client, err := artifactory.NewClient(
				*artifactoryProviderHostname,
				*artifactoryProviderUsername,
				*artifactoryProviderPassword,
				*artifactoryProviderAccessToken,
				*artifactoryProviderAPIKey)
			if err != nil {
				logger.Fatal("initialize artifactory", zap.Error(err))
			}
			providers = append(providers, artifactory.New(client,
				artifactory.WithSessionDuration(*artifactoryProviderSessionDuration),
			))
		}

		var store sidecred.SecretStore
		switch sidecred.StoreType(*secretStoreBackend) {
		case sidecred.SecretsManager:
			_, _, _, client := newAWSClient()
			store = secretsmanager.New(client,
				secretsmanager.WithPathTemplate(*secretsManagerStorePathTemplate),
			)
		case sidecred.SSM:
			_, _, client, _ := newAWSClient()
			store = ssm.New(client,
				ssm.WithPathTemplate(*ssmStorePathTemplate),
				ssm.WithKMSKeyID(*ssmStoreKMSKeyID),
			)
		case sidecred.Inprocess:
			store = inprocess.New(
				inprocess.WithPathTemplate(*inprocessStorePathTemplate),
			)
		default:
			logger.Fatal("unknown secretstore backend", zap.String("backend", *secretStoreBackend))
		}

		var backend sidecred.StateBackend
		switch *stateBackend {
		case "file":
			backend = file.New()
		case "s3":
			client, _, _, _ := newAWSClient()
			backend = s3.New(client, *s3BackendBucket)
		default:
			logger.Fatal("unknown state backend", zap.String("backend", *stateBackend))
		}

		s, err := sidecred.New(providers, store, *rotationWindow, logger)
		if err != nil {
			logger.Fatal("initialize sidecred", zap.Error(err))
		}
		return run(s, backend)
	})
}

func defaultAWSClientFactory() (s3.S3API, sts.STSAPI, ssm.SSMAPI, secretsmanager.SecretsManagerAPI) {
	var (
		sess *session.Session
		err  error
		once sync.Once
	)
	once.Do(func() {
		sess, err = session.NewSession(&aws.Config{Region: aws.String(os.Getenv("AWS_REGION"))})
		if err != nil {
			panic(fmt.Errorf("create aws session: %s", err))
		}
	})
	return s3.NewClient(sess), sts.NewClient(sess), ssm.NewClient(sess), secretsmanager.NewClient(sess)
}

func defaultLogger(debug bool) (*zap.Logger, error) {
	config := zap.NewProductionConfig()

	// Disable entries like: "caller":"autoapprover/autoapprover.go:97"
	config.DisableCaller = true

	// Disable logging the stack trace
	config.DisableStacktrace = true

	// Format timestamps as RFC3339 strings
	// Adapted from: https://github.com/uber-go/zap/issues/661#issuecomment-520686037
	config.EncoderConfig.EncodeTime = zapcore.TimeEncoder(
		func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(t.UTC().Format(time.RFC3339))
		},
	)

	if debug {
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	}

	return config.Build()
}
