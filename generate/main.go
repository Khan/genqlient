package generate

import (
	"fmt"
	"io"
	"os"
)

func outputWriter(filename string) (io.Writer, error) {
	if filename == "-" {
		return os.Stdout, nil
	}

	f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return nil, fmt.Errorf("could not open generated file %v: %v",
			filename, err)
	}
	return f, nil
}

func ParseGenerateAndWrite(specFilename, schemaFilename string, out io.Writer) error {
	schema, err := getSchema(schemaFilename)
	if err != nil {
		return err
	}

	document, err := getAndValidateQueries(specFilename, schema)
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

	if len(os.Args) != 4 {
		err = fmt.Errorf("usage: %s queries.graphql schema.graphql generated.go",
			os.Args[0])
		return
	}

	out, err := outputWriter(os.Args[3])
	if err != nil {
		return
	}

	err = ParseGenerateAndWrite(os.Args[1], os.Args[2], out)
}
