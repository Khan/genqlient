package generate

import (
	"bytes"
	"fmt"
	"go/format"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"text/template"

	"github.com/vektah/gqlparser"
	"github.com/vektah/gqlparser/ast"
	"github.com/vektah/gqlparser/formatter"
	"github.com/vektah/gqlparser/parser"
	"github.com/vektah/gqlparser/validator"
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
	// The body of the operation to send.
	Operation string
}

func Generate(specFilename, schemaFilename, generatedFilename string) error {
	// TODO: all the read-parse-and-validate stuff can probably get factored
	// out a bit
	// TODO: IRL we have to get the schema from GraphQL (maybe we can generate
	// that once we can bootstrap) where it comes as JSON, not SDL, so we have
	// to convert (or add gqlparser support to convert)
	text, err := ioutil.ReadFile(schemaFilename)
	if err != nil {
		return fmt.Errorf("unreadable schema file %v: %v",
			schemaFilename, err)
	}

	schema, graphqlError := gqlparser.LoadSchema(
		&ast.Source{Name: schemaFilename, Input: string(text)})
	if graphqlError != nil {
		return fmt.Errorf("invalid schema file %v: %v",
			schemaFilename, graphqlError)
	}

	text, err = ioutil.ReadFile(specFilename)
	if err != nil {
		return fmt.Errorf("unreadable query-spec file %v: %v",
			specFilename, err)
	}

	// The following is more or less gqlparser.LoadQuery, but we can provide a
	// name so we might as well (and we break out the two errors).
	document, graphqlError := parser.ParseQuery(
		&ast.Source{Name: specFilename, Input: string(text)})
	if graphqlError != nil { // ParseQuery returns type *graphql.Error, yuck
		return fmt.Errorf("invalid query-spec file %v: %v",
			specFilename, graphqlError)
	}

	graphqlErrors := validator.Validate(schema, document)
	if graphqlErrors != nil {
		return fmt.Errorf("query-spec does not match schema: %v", graphqlErrors)
	}

	var out io.Writer
	if generatedFilename == "-" {
		out = os.Stdout
	} else {
		out, err = os.OpenFile(generatedFilename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return fmt.Errorf("could not open generated file %v: %v",
				generatedFilename, err)
		}
	}

	// TODO: configure this
	packageName := "example"

	// TODO: this should probably get factored out
	operations := make([]OperationParams, len(document.Operations))
	for i, operation := range document.Operations {
		// TODO: we may have to actually get the precise query text, in case we
		// want to be hashing it or something like that.  Although maybe
		// there's no reasonable way to do that with several queries in one
		// file.
		var builder strings.Builder
		f := formatter.NewFormatter(&builder)
		f.FormatQueryDocument(&ast.QueryDocument{
			Operations: ast.OperationList{operation},
			// TODO: handle fragments
		})
		operations[i] = OperationParams{
			OperationType: operation.Operation,
			OperationName: operation.Name,
			// TODO: this is actually awkward, because GraphQL doesn't allow
			// for docstrings on queries (only schemas).  So we have to extract
			// the comment, or omit doc-comments for now.
			OperationDoc: "TODO",

			// TODO: configure ResponseName format
			ResponseName: operation.Name + "Response",
			ResponseType: typeFor(operation, schema),

			// The newline just makes it format a little nicer
			Operation: "\n" + builder.String(),
		}
	}

	data := TemplateParams{
		PackageName: packageName,
		Operations:  operations,
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return fmt.Errorf("could not render template: %v", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("could not gofmt template: %v", err)
	}

	_, err = out.Write(formatted)
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
	err = Generate(os.Args[1], os.Args[2], os.Args[3])
}
