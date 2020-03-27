package generate

import (
	"fmt"
	"os"
)

func readConfigGenerateAndWrite(configFilename string) error {
	config, err := ReadAndValidateConfig(configFilename)
	if err != nil {
		return err
	}

	out, err := os.OpenFile(config.Generated, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("could not open generated file %v: %v",
			config.Generated, err)
	}

	code, err := Generate(config)
	if err != nil {
		return err
	}

	_, err = out.Write(code)
	return err
}

func Main() {
	var err error
	defer func() {
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}()

	if len(os.Args) != 2 {
		// TODO: omit config to get it from genql.yaml, or to use the defaults.
		err = fmt.Errorf("usage: %s genql.yaml", os.Args[0])
		return
	}

	err = readConfigGenerateAndWrite(os.Args[1])
}
