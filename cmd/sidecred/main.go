package main

import (
	"context"
	"fmt"
	"os"

	"github.com/alecthomas/kingpin"

	"github.com/telia-oss/sidecred"
	"github.com/telia-oss/sidecred/config"
	"github.com/telia-oss/sidecred/internal/cli"
)

var version string

func main() {
	var (
		app        = kingpin.New("sidecred", "Sideload your credentials.").Version(version).UsageWriter(os.Stdout).ErrorWriter(os.Stdout).DefaultEnvars()
		configPath = app.Flag("config", "Path to the config file containing the requests").ExistingFile()
		statePath  = app.Flag("state", "Path to use for storing state in a file backend").Default("state.json").String()
	)
	cli.AddRunCommand(app, runFunc(configPath, statePath), nil, nil).Default()

	validate := app.Command("validate", "Validate a sidecred config.")
	validate.Action(func(_ *kingpin.ParseContext) error {
		b, err := os.ReadFile(*configPath)
		if err != nil {
			app.Fatalf("failed to read config: %s", err)
		}
		cfg, err := config.Parse(b)
		if err != nil {
			app.Fatalf("failed to parse config: %s", err)
		}
		if err := cfg.Validate(); err != nil {
			app.Fatalf("validate: %s", err)
		}
		return nil
	})

	kingpin.MustParse(app.Parse(os.Args[1:]))
}

func runFunc(cfg, statePath *string) func(*sidecred.Sidecred, sidecred.StateBackend, sidecred.RunConfig) error {
	return func(s *sidecred.Sidecred, backend sidecred.StateBackend, runConfig sidecred.RunConfig) error {
		ctx := context.Background()

		b, err := os.ReadFile(*cfg)
		if err != nil {
			return fmt.Errorf("failed to read config: %s", err)
		}
		cfg, err := config.Parse(b)
		if err != nil {
			return fmt.Errorf("failed to parse config: %s", err)
		}
		state, err := backend.Load(ctx, *statePath)
		if err != nil {
			return fmt.Errorf("failed to load state: %s", err)
		}
		if err := s.Process(ctx, cfg, state); err != nil {
			return err
		}
		if err := backend.Save(ctx, *statePath, state); err != nil {
			return fmt.Errorf("failed to save state: %s", err)
		}
		return nil
	}
}
