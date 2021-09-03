package generate

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
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

func Main() {
	var err error
	defer func() {
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}()

	switch len(os.Args) {
	case 2:
		err = readConfigGenerateAndWrite(os.Args[1])
	case 1:
		err = readConfigGenerateAndWrite("genqlient.yaml")
	default:
		argv0 := os.Args[0]
		if strings.Contains(argv0, string(filepath.Separator)+"go-build") {
			argv0 = "go run github.com/Khan/genqlient"
		}
		err = errorf(nil, "usage: %s [config]", argv0)
	}
}
