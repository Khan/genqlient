package generate

import (
	"fmt"
	"os"
)

func parseGenerateAndWrite(configFilename string) error {
	config, err := ReadAndValidateConfig(configFilename)
	if err != nil {
		return err
	}

	out, err := os.OpenFile(config.Generated, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("could not open generated file %v: %v",
			config.Generated, err)
	}

	schema, err := getSchema(config.Schema)
	if err != nil {
		return err
	}

	document, err := getAndValidateQueries(config.Queries, schema)
	if err != nil {
		return err
	}

	code, err := Generate(schema, document)
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

	err = parseGenerateAndWrite(os.Args[1])
}
