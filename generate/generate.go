package generate

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"text/template"

	"github.com/vektah/gqlparser/ast"
	"github.com/vektah/gqlparser/formatter"
	"github.com/vektah/gqlparser/parser"
)

// TODO: package template into the binary using one of those asset thingies
const tmplFilename = "generate/operation.go.tmpl"

var tmpl = template.Must(template.ParseFiles(tmplFilename))

type TemplateParams struct {
	// The name of the package into which to generate the operation-helpers.
	PackageName string
	// The list of operations for which to generate code.
	Operations []OperationParams
}

type OperationParams struct {
	// The type-name for the operation's response type.
	ResponseName string
	// The body of the operation's response type (e.g. struct { ... }).
	ResponseType string
	// The type of the operation (query, mutation, or subscription).
	OperationType ast.Operation
	// The name of the operation, from GraphQL.
	OperationName string
	// The documentation for the operation, from GraphQL.
	OperationDoc string
	// The endpoint to which to send queries.
	Endpoint string
	// The body of the operation to send.
	Operation string
}

func Generate(specFilename, generatedFilename string) error {
	text, err := ioutil.ReadFile(specFilename)
	if err != nil {
		return fmt.Errorf("could not open query-spec file %v: %v",
			specFilename, err)
	}

	document, graphqlError := parser.ParseQuery(
		&ast.Source{Name: specFilename, Input: string(text)})
	if graphqlError != nil { // ParseQuery returns type *graphql.Error, yuck
		return fmt.Errorf("could not parse query-spec file %v: %v",
			specFilename, graphqlError)
	}

	var out io.Writer
	if generatedFilename == "-" {
		out = os.Stdout
	} else {
		out, err = os.OpenFile(generatedFilename, os.O_RDWR|os.O_CREATE, 0755)
		if err != nil {
			return fmt.Errorf("could not open generated file %v: %v",
				generatedFilename, err)
		}
	}

	// TODO: configure these
	packageName := "example"
	endpoint := "https://api.github.com/graphql"

	operations := make([]OperationParams, len(document.Operations))
	for i, operation := range document.Operations {
		var builder strings.Builder
		f := formatter.NewFormatter(&builder)
		f.FormatQueryDocument(&ast.QueryDocument{
			Operations: ast.OperationList{operation},
			// TODO: handle fragments
		})
		operations[i] = OperationParams{
			OperationType: operation.Operation,
			OperationName: operation.Name,
			OperationDoc:  "TODO",

			// TODO: configure this
			ResponseName: operation.Name + "Response",
			ResponseType: "struct{} // TODO",

			Endpoint: endpoint,
			// The newline just makes it format a little nicer
			Operation: "\n" + builder.String(),
		}
	}

	data := TemplateParams{
		PackageName: packageName,
		Operations:  operations,
	}

	err = tmpl.Execute(out, data)
	if err != nil {
		return fmt.Errorf("could not render template: %v", err)
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

	if len(os.Args) != 3 {
		err = fmt.Errorf("usage: %s queries.graphql generated.go", os.Args[0])
		return
	}
	err = Generate(os.Args[1], os.Args[2])
}
