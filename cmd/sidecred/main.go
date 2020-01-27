package main

import (
	"os"
	"time"

	"github.com/alecthomas/kingpin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	version string
)

func main() {
	app := kingpin.New("sidecred", "Sideload your credentials.")
	app.Version(version).Writer(os.Stdout).DefaultEnvars()
	kingpin.MustParse(app.Parse(os.Args[1:]))
}

func newLogger(debug bool) (*zap.Logger, error) {
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
