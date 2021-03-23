package generate

import (
	"fmt"
	"io/ioutil"
	"os"
)

func readConfigGenerateAndWrite(configFilename string) error {
	config, err := ReadAndValidateConfig(configFilename)
	if err != nil {
		return err
	}

	code, err := Generate(config)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(config.Generated, code, 0o644)
	if err != nil {
		return fmt.Errorf("could not write generated file %v: %v",
			config.Generated, err)
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
		err = readConfigGenerateAndWrite("")
	default:
		err = fmt.Errorf("usage: %s [config]", os.Args[0])
	}
}
