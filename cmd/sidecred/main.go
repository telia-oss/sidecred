package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/telia-oss/sidecred"
	"github.com/telia-oss/sidecred/config"
	"github.com/telia-oss/sidecred/internal/cli"

	"github.com/alecthomas/kingpin"
)

var version string

func main() {
	var (
		app        = kingpin.New("sidecred", "Sideload your credentials.").Version(version).Writer(os.Stdout).DefaultEnvars()
		configPath = app.Flag("config", "Path to the config file containing the requests").ExistingFile()
		statePath  = app.Flag("state", "Path to use for storing state in a file backend").Default("state.json").String()
	)
	cli.AddRunCommand(app, runFunc(configPath, statePath), nil, nil).Default()

	validate := app.Command("validate", "Validate a sidecred config.")
	validate.Action(func(_ *kingpin.ParseContext) error {
		b, err := ioutil.ReadFile(*configPath)
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

func runFunc(cfg *string, statePath *string) func(*sidecred.Sidecred, sidecred.StateBackend) error {
	return func(s *sidecred.Sidecred, backend sidecred.StateBackend) error {
		b, err := ioutil.ReadFile(*cfg)
		if err != nil {
			return fmt.Errorf("failed to read config: %s", err)
		}
		cfg, err := config.Parse(b)
		if err != nil {
			return fmt.Errorf("failed to parse config: %s", err)
		}
		state, err := backend.Load(*statePath)
		if err != nil {
			return fmt.Errorf("failed to load state: %s", err)
		}
		defer backend.Save(*statePath, state)
		return s.Process(cfg, state)
	}
}
