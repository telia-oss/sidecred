package main

import (
	"io/ioutil"
	"os"

	"github.com/telia-oss/sidecred"
	"github.com/telia-oss/sidecred/internal/cli"

	"github.com/alecthomas/kingpin"
	"sigs.k8s.io/yaml"
)

var version string

func main() {
	var (
		app       = kingpin.New("sidecred", "Sideload your credentials.").Version(version).Writer(os.Stdout).DefaultEnvars()
		namespace = app.Flag("namespace", "Namespace to use when processing the requests.").Required().String()
		config    = app.Flag("config", "Path to the config file containing the requests").ExistingFile()
	)
	cli.Setup(app, runFunc(namespace, config), nil, nil)
	kingpin.MustParse(app.Parse(os.Args[1:]))
}

func runFunc(namespace *string, config *string) func(*sidecred.Sidecred) error {
	return func(s *sidecred.Sidecred) error {
		b, err := ioutil.ReadFile(*config)
		if err != nil {
			return err
		}
		var requests []*sidecred.Request
		if err := yaml.UnmarshalStrict(b, &requests); err != nil {
			return err
		}
		return s.Process(*namespace, requests)
	}
}
