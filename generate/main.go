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

	code, err := Generate(config)
	if err != nil {
		return err
	}

	// Open out at the end -- decreases the chances we blank it if we err.
	out, err := os.OpenFile(config.Generated, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("could not open generated file %v: %v",
			config.Generated, err)
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

	switch len(os.Args) {
	case 2:
		err = readConfigGenerateAndWrite(os.Args[1])
	case 1:
		err = readConfigGenerateAndWrite("")
	default:
		err = fmt.Errorf("usage: %s [config]", os.Args[0])
	}
}
