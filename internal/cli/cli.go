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
	githubstore "github.com/telia-oss/sidecred/store/github"
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

// AddRunCommand configures a kingpin.Application to run sidecred.
func AddRunCommand(app *kingpin.Application, run runFunc, newAWSClient awsClientFactory, newLogger loggerFactory) *kingpin.CmdClause {
	var (
		cmd                                = app.Command("run", "Run sidecred.")
		randomProviderRotationInterval     = cmd.Flag("random-provider-rotation-interval", "Rotation interval for the random provider").Default("168h").Duration()
		stsProviderEnabled                 = cmd.Flag("sts-provider-enabled", "Enable the STS provider").Bool()
		stsProviderExternalID              = cmd.Flag("sts-provider-external-id", "External ID for the STS Provider").String()
		stsProviderSessionDuration         = cmd.Flag("sts-provider-session-duration", "Session duration for STS credentials").Default("1h").Duration()
		githubProviderEnabled              = cmd.Flag("github-provider-enabled", "Enable the Github provider").Bool()
		githubProviderIntegrationID        = cmd.Flag("github-provider-integration-id", "Github Apps integration ID").Int64()
		githubProviderPrivateKey           = cmd.Flag("github-provider-private-key", "Github apps private key").String()
		githubProviderKeyRotationInterval  = cmd.Flag("github-provider-key-rotation-interval", "Rotation interval for deploy keys").Default("168h").Duration()
		artifactoryProviderEnabled         = cmd.Flag("artifactory-provider-enabled", "Enable the Artifactory provider").Bool()
		artifactoryProviderHostname        = cmd.Flag("artifactory-provider-hostname", "Hostname for the Artifactory Provider").String()
		artifactoryProviderUsername        = cmd.Flag("artifactory-provider-username", "Username for the Artifactory Provider").String()
		artifactoryProviderPassword        = cmd.Flag("artifactory-provider-password", "Password for the Artifactory Provider").String()
		artifactoryProviderAccessToken     = cmd.Flag("artifactory-provider-access-token", "Access token for the Artifactory Provider").String()
		artifactoryProviderAPIKey          = cmd.Flag("artifactory-provider-api-key", "API key for the Artifactory Provider").String()
		artifactoryProviderSessionDuration = cmd.Flag("artifactory-provider-session-duration", "Session duration for artifactory tokens").Default("1h").Duration()
		inprocessStoreSecretTemplate       = cmd.Flag("inprocess-store-secret-template", "Path template to use for the inprocess store").Default("{{ .Namespace }}.{{ .Name }}").String()
		secretsManagerStoreEnabled         = cmd.Flag("secrets-manager-store-enabled", "Enable AWS Secrets Manager store for secrets").Bool()
		secretsManagerStoreSecretTemplate  = cmd.Flag("secrets-manager-store-secret-template", "Path template to use for the secrets manager store").Default("/{{ .Namespace }}/{{ .Name }}").String()
		ssmStoreEnabled                    = cmd.Flag("ssm-store-enabled", "Enable AWS SSM Parameter store for secrets").Bool()
		ssmStoreSecretTemplate             = cmd.Flag("ssm-store-secret-template", "Path template to use for SSM Parameter store").Default("/{{ .Namespace }}/{{ .Name }}").String()
		ssmStoreKMSKeyID                   = cmd.Flag("ssm-store-kms-key-id", "KMS key to use for encrypting secrets stored in SSM Parameter store").String()
		githubStoreEnabled                 = cmd.Flag("github-store-enabled", "Enable Github repository secrets store").Bool()
		githubStoreSecretTemplate          = cmd.Flag("github-store-secret-template", "Template to use for naming Github repository secrets").Default("{{ .Namespace}}_{{ .Name }}").String()
		githubStoreIntegrationID           = cmd.Flag("github-store-integration-id", "Github Apps integration ID").Int64()
		githubStorePrivateKey              = cmd.Flag("github-store-private-key", "Github apps private key").String()
		stateBackend                       = cmd.Flag("state-backend", "Backend to use for storing state").Required().String()
		s3BackendBucket                    = cmd.Flag("s3-backend-bucket", "Bucket name to use for the S3 state backend").String()
		rotationWindow                     = cmd.Flag("rotation-window", "A window in time (duration) where sidecred should rotate credentials prior to their expiration").Default("10m").Duration()
		debug                              = cmd.Flag("debug", "Enable debug logging").Bool()
	)

	cmd.Action(func(_ *kingpin.ParseContext) error {
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

		var providers []sidecred.Provider
		{
			p := random.New(time.Now().UnixNano(), random.Options{
				RotationInterval: *randomProviderRotationInterval,
			})
			providers = append(providers, p)
		}
		if *stsProviderEnabled {
			_, c, _, _ := newAWSClient()
			p := sts.New(c, sts.Options{
				ExternalID:      *stsProviderExternalID,
				SessionDuration: *stsProviderSessionDuration,
			})
			providers = append(providers, p)
		}
		if *githubProviderEnabled {
			c, err := githubapp.NewClient(*githubProviderIntegrationID, []byte(*githubProviderPrivateKey))
			if err != nil {
				logger.Fatal("initialize github provider app", zap.Error(err))
			}
			p := github.New(githubapp.New(c), github.Options{
				DeployKeyRotationInterval: *githubProviderKeyRotationInterval,
			})
			providers = append(providers, p)
		}
		if *artifactoryProviderEnabled {
			c, err := artifactory.NewClient(
				*artifactoryProviderHostname,
				*artifactoryProviderUsername,
				*artifactoryProviderPassword,
				*artifactoryProviderAccessToken,
				*artifactoryProviderAPIKey,
			)
			if err != nil {
				logger.Fatal("initialize artifactory", zap.Error(err))
			}
			p := artifactory.New(c, artifactory.Options{
				SessionDuration: *artifactoryProviderSessionDuration,
			})
			providers = append(providers, p)
		}

		stores := []sidecred.SecretStore{inprocess.New(
			inprocess.WithSecretTemplate(*inprocessStoreSecretTemplate),
		)}
		if *secretsManagerStoreEnabled {
			_, _, _, client := newAWSClient()
			stores = append(stores, secretsmanager.New(client,
				secretsmanager.WithSecretTemplate(*secretsManagerStoreSecretTemplate),
			))
		}
		if *ssmStoreEnabled {
			_, _, client, _ := newAWSClient()
			stores = append(stores, ssm.New(client,
				ssm.WithSecretTemplate(*ssmStoreSecretTemplate),
				ssm.WithKMSKeyID(*ssmStoreKMSKeyID),
			))
		}
		if *githubStoreEnabled {
			client, err := githubapp.NewClient(*githubStoreIntegrationID, []byte(*githubStorePrivateKey))
			if err != nil {
				logger.Fatal("initialize github store app", zap.Error(err))
			}
			stores = append(stores, githubstore.New(
				githubapp.New(client),
				githubstore.WithSecretTemplate(*githubStoreSecretTemplate),
			))
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

		s, err := sidecred.New(providers, stores, *rotationWindow, logger)
		if err != nil {
			logger.Fatal("initialize sidecred", zap.Error(err))
		}
		if err := run(s, backend); err != nil {
			logger.Fatal("run failed", zap.Error(err))
		}
		return nil
	})
	return cmd
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
	config.EncoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.UTC().Format(time.RFC3339))
	}

	if debug {
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	}

	return config.Build()
}
