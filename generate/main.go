package generate

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/alexflint/go-arg"
)

func readConfigGenerateAndWrite(configFilename string) error {
	config, err := ReadAndValidateConfig(configFilename)
	if err != nil {
		return err
	}

	generated, err := Generate(config)
	if err != nil {
		return err
	}

	for filename, content := range generated {
		err = os.MkdirAll(filepath.Dir(filename), 0o755)
		if err != nil {
			return errorf(nil,
				"could not create parent directory for generated file %v: %v",
				filename, err)
		}

		err = ioutil.WriteFile(filename, content, 0o644)
		if err != nil {
			return errorf(nil, "could not write generated file %v: %v",
				filename, err)
		}
	}
	return nil
}

type cliArgs struct {
	ConfigFilename string `arg:"positional" placeholder:"FILENAME" default:"genqlient.yaml" help:"path to genqlient configuration (default genqlient.yaml)"`
	Init           bool   `arg:"--init" help:"write out and use a default config file"`
}

func (cliArgs) Description() string {
	return strings.TrimSpace(`
Generates GraphQL client code for a given schema and queries.
See https://github.com/Khan/genqlient for full documentation.
`)
}

func Main() {
	exitIfError := func(err error) {
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	var args cliArgs
	arg.MustParse(&args)
	if args.Init {
		err := initConfig(args.ConfigFilename)
		exitIfError(err)
	}
	err := readConfigGenerateAndWrite(args.ConfigFilename)
	exitIfError(err)
}
