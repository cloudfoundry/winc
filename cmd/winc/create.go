package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"unicode/utf8"

	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/hcsclient"
	"code.cloudfoundry.org/winc/sandbox"

	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/validate"
	"github.com/urfave/cli"
)

var createCommand = cli.Command{
	Name:  "create",
	Usage: "create a container",
	ArgsUsage: `<container-id>

Where "<container-id>" is your name for the instance of the container that you
are starting. The name you provide for the container instance must be unique on
your host.`,
	Description: `The create command creates an instance of a container for a bundle. The bundle
	is a directory with a specification file named "` + specConfig + `" and a root
	filesystem.

	The specification file includes an args parameter. The args parameter is used
	to specify command(s) that get run when the container is started. To change the
	command(s) that get executed on start, edit the args parameter of the spec`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "bundle, b",
			Value: "",
			Usage: `path to the root of the bundle directory, defaults to the current directory`,
		},
	},
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 1, exactArgs); err != nil {
			return err
		}

		bundlePath := context.String("bundle")
		containerId := context.Args().First()

		if bundlePath == "" {
			var err error
			bundlePath, err = os.Getwd()
			if err != nil {
				return err
			}
		}

		if _, err := os.Stat(bundlePath); err != nil {
			return err
		}

		configPath := filepath.Join(bundlePath, specConfig)
		content, err := ioutil.ReadFile(configPath)
		if err != nil {
			return err
		}
		if !utf8.Valid(content) {
			return fmt.Errorf("%q is not encoded in UTF-8", configPath)
		}
		var spec specs.Spec
		if err = json.Unmarshal(content, &spec); err != nil {
			return err
		}

		validator := validate.NewValidator(&spec, bundlePath, true)

		m := validator.CheckMandatoryFields()
		if len(m) != 0 {
			return &WincBundleConfigValidationError{m}
		}

		client := hcsclient.HCSClient{}
		sm := sandbox.NewManager(&client, bundlePath)
		cm := container.NewManager(&client, sm, containerId)

		return cm.Create(spec.Root.Path)
	},
}
