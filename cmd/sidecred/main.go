package main

import (
	"io/ioutil"
	"os"
	"time"

	"github.com/telia-oss/sidecred"
	"github.com/telia-oss/sidecred/internal/cli"

	"github.com/alecthomas/kingpin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"sigs.k8s.io/yaml"
)

var (
	version string
)

func main() {
	app := kingpin.New("sidecred", "Sideload your credentials.").Version(version).Writer(os.Stdout).DefaultEnvars()
	var (
		namespace = app.Flag("namespace", "Namespace to use when processing the requests.").Required().String()
		config    = app.Flag("config", "Path to the config file containing the requests").ExistingFile()
	)
	cli.Setup(app, runFunc(namespace, config), loggerFactory)
	kingpin.MustParse(app.Parse(os.Args[1:]))
}

func runFunc(namespace *string, config *string) func(func(namespace string, requests []*sidecred.Request) error) error {
	return func(f func(namespace string, requests []*sidecred.Request) error) error {
		b, err := ioutil.ReadFile(*config)
		if err != nil {
			return err
		}
		var requests []*sidecred.Request
		if err := yaml.UnmarshalStrict(b, &requests); err != nil {
			return err
		}
		return f(*namespace, requests)
	}
}

func loggerFactory(debug bool) (*zap.Logger, error) {
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
