package cli

import (
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
	"sigs.k8s.io/yaml"
)

type (
	loggerFactory func(bool) (*zap.Logger, error)
)

// Setup a kingpin.Application to run the autoapprover.
func Setup(app *kingpin.Application, loggerFactory loggerFactory) {
	var (
		namespace                         = app.Flag("namespace", "Namespace to use when processing the requests.").Required().String()
		config                            = app.Flag("config", "Path to the config file containing the requests").ExistingFile()
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
		logger, err := loggerFactory(*debug)
		if err != nil {
			kingpin.Fatalf("initialize logger: %s", err)
		}
		defer logger.Sync()

		var (
			requests  []*sidecred.Request
			providers []sidecred.Provider
			store     sidecred.SecretStore
			backend   sidecred.StateBackend
		)

		providers = append(providers, random.New(time.Now().UnixNano()))

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
				kingpin.Fatalf("read private key: %s", err)
			}
			app, err := github.NewAppsClient(*githubProviderIntegrationID, string(privateKey))
			if err != nil {
				kingpin.Fatalf("initialize github app: %s", err)
			}
			providers = append(providers, github.New(app,
				github.WithDeployKeyRotationInterval(*githubProviderKeyRotationInterval),
			))
		}

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
			kingpin.Fatalf("unknown secretstore backend: %s", *secretStoreBackend)
		}

		switch *stateBackend {
		case "file":
			backend = file.New(*fileBackendPath)
		case "s3":
			backend = s3.New(s3.NewClient(newAWSSession()), *s3BackendBucket, *s3BackendPath)
		default:
			kingpin.Fatalf("unknown state backend: %s", *stateBackend)
		}

		requests, err = loadConfigFromFile(*config)
		if err != nil {
			kingpin.Fatalf("load config: %s", err)
		}
		s, err := sidecred.New(providers, store, backend, logger)
		if err != nil {
			kingpin.Fatalf("initialize sidecred: %s", err)
		}
		return s.Process(*namespace, requests)
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

func loadConfigFromFile(f string) ([]*sidecred.Request, error) {
	b, err := ioutil.ReadFile(f)
	if err != nil {
		return nil, err
	}
	var config []*sidecred.Request
	if err := yaml.UnmarshalStrict(b, &config); err != nil {
		return nil, err
	}
	return config, nil
}
