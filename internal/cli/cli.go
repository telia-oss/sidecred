package cli

import (
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"time"

	"github.com/telia-oss/sidecred"
	"github.com/telia-oss/sidecred/backend/file"
	"github.com/telia-oss/sidecred/backend/s3"
	"github.com/telia-oss/sidecred/provider/github"
	"github.com/telia-oss/sidecred/provider/random"
	"github.com/telia-oss/sidecred/provider/sts"
	"github.com/telia-oss/sidecred/store/inprocess"
	"github.com/telia-oss/sidecred/store/secretsmanager"
	"github.com/telia-oss/sidecred/store/ssm"

	"github.com/alecthomas/kingpin"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"go.uber.org/zap"
)

type (
	runFunc       func(func(namespace string, requests []*sidecred.Request) error) error
	loggerFactory func(bool) (*zap.Logger, error)
)

// Setup a kingpin.Application to run the autoapprover.
func Setup(app *kingpin.Application, run runFunc, newLogger loggerFactory) {
	var (
		stsProviderEnabled                = app.Flag("sts-provider-enabled", "Enable the STS provider").Bool()
		stsProviderExternalID             = app.Flag("sts-provider-external-id", "External ID for the STS Provider").String()
		stsProviderSessionDuration        = app.Flag("sts-provider-session-duration", "Session duration for STS credentials").Default("1h").Duration()
		githubProviderEnabled             = app.Flag("github-provider-enabled", "Enable the Github provider").Bool()
		githubProviderIntegrationID       = app.Flag("github-provider-integration-id", "Github Apps integration ID").Int64()
		githubProviderPrivateKey          = app.Flag("github-provider-private-key", "Github apps private key").String()
		githubProviderKeyRotationInterval = app.Flag("github-provider-key-rotation-interval", "Rotation interval for deploy keys").Default("168h").Duration()
		secretStoreBackend                = app.Flag("secret-store-backend", "Backend to use for secrets").Required().String()
		inprocessStorePathTemplate        = app.Flag("inprocess-store-path-template", "Path template to use for the inprocess store").Default("{{ .Namespace }}.{{ .Name }}").String()
		secretsManagerStorePathTemplate   = app.Flag("secrets-manager-store-path-template", "Path template to use for the secrets manager store").Default("/{{ .Namespace }}/{{ .Name }}").String()
		ssmStorePathTemplate              = app.Flag("ssm-store-path-template", "Path template to use for SSM Parameter store").Default("/{{ .Namespace }}/{{ .Name }}").String()
		ssmStoreKMSKeyID                  = app.Flag("ssm-store-kms-key-id", "KMS key to use for encrypting secrets stored in SSM Parameter store").String()
		stateBackend                      = app.Flag("state-backend", "Backend to use for storing state").Required().String()
		fileBackendPath                   = app.Flag("file-backend-path", "Path to use for storing state in a file backend").Default("state.json").String()
		s3BackendBucket                   = app.Flag("s3-backend-bucket", "Bucket name to use for the S3 state backend").String()
		s3BackendPath                     = app.Flag("s3-backend-path", "Path to use when storing state in the S3 state backend").String()
		debug                             = app.Flag("debug", "Enable debug logging").Bool()
	)

	app.Action(func(_ *kingpin.ParseContext) error {
		logger, err := newLogger(*debug)
		if err != nil {
			panic(fmt.Errorf("initialize zap logger: %s", err))
		}
		defer logger.Sync()

		providers := []sidecred.Provider{random.New(time.Now().UnixNano())}

		if *stsProviderEnabled {
			providers = append(providers, sts.New(
				sts.NewClient(newAWSSession()),
				sts.WithExternalID(*stsProviderExternalID),
				sts.WithSessionDuration(*stsProviderSessionDuration),
			))
		}

		if *githubProviderEnabled {
			privateKey, err := ioutil.ReadFile(*githubProviderPrivateKey)
			if err != nil {
				logger.Fatal("read private key", zap.Error(err))
			}
			app, err := github.NewAppsClient(*githubProviderIntegrationID, string(privateKey))
			if err != nil {
				logger.Fatal("initialize github app", zap.Error(err))
			}
			providers = append(providers, github.New(app,
				github.WithDeployKeyRotationInterval(*githubProviderKeyRotationInterval),
			))
		}

		var store sidecred.SecretStore
		switch sidecred.StoreType(*secretStoreBackend) {
		case sidecred.SecretsManager:
			store = secretsmanager.New(
				secretsmanager.NewClient(newAWSSession()),
				secretsmanager.WithPathTemplate(*secretsManagerStorePathTemplate),
			)
		case sidecred.SSM:
			store = ssm.New(
				ssm.NewClient(newAWSSession()),
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
			backend = file.New(*fileBackendPath)
		case "s3":
			backend = s3.New(s3.NewClient(newAWSSession()), *s3BackendBucket, *s3BackendPath)
		default:
			logger.Fatal("unknown state backend", zap.String("backend", *stateBackend))
		}

		s, err := sidecred.New(providers, store, backend, logger)
		if err != nil {
			logger.Fatal("initialize sidecred", zap.Error(err))
		}
		return run(s.Process)
	})
}

func newAWSSession() *session.Session {
	var (
		sess *session.Session
		err  error
		once sync.Once
	)
	once.Do(func() {
		sess, err = session.NewSession(&aws.Config{Region: aws.String(os.Getenv("AWS_REGION"))})
		if err != nil {
			kingpin.Fatalf("create aws session: %s", err)
		}
	})
	return sess
}
