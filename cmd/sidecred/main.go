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
		namespace  = app.Flag("namespace", "Namespace to use when processing the requests.").Required().String()
		configPath = app.Flag("config", "Path to the config file containing the requests").ExistingFile()
		statePath  = app.Flag("state", "Path to use for storing state in a file backend").Default("state.json").String()
	)
	cli.Setup(app, runFunc(namespace, configPath, statePath), nil, nil)
	kingpin.MustParse(app.Parse(os.Args[1:]))
}

func runFunc(namespace *string, cfg *string, statePath *string) func(*sidecred.Sidecred, sidecred.StateBackend) error {
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
