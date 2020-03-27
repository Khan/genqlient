package generate

import (
	"bytes"
	"fmt"
	"go/format"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"github.com/vektah/gqlparser/ast"
	"github.com/vektah/gqlparser/formatter"
)

// TODO: package template into the binary using one of those asset thingies
var _, thisFilename, _, _ = runtime.Caller(0)
var tmplRelFilename = "operation.go.tmpl"
var tmplAbsFilename = filepath.Join(filepath.Dir(thisFilename), tmplRelFilename)

var tmpl = template.Must(template.ParseFiles(tmplAbsFilename))

type templateParams struct {
	// The name of the package into which to generate the operation-helpers.
	PackageName string
	// The list of operations for which to generate code.
	Operations []operation
}

type operation struct {
	// The type of the operation (query, mutation, or subscription).
	Type ast.Operation
	// The name of the operation, from GraphQL.
	Name string
	// The documentation for the operation, from GraphQL.
	Doc string
	// The body of the operation to send.
	Body string
	// The arguments to the operation.
	Args []argument

	// The type-name for the operation's response type.
	ResponseName string
	// The body of the operation's response type (e.g. struct { ... }).
	ResponseType string
}

type argument struct {
	GoName      string
	GoType      string
	GraphQLName string
}

func fromASTArg(arg *ast.VariableDefinition, schema *ast.Schema) argument {
	return argument{
		GraphQLName: arg.Variable,
		GoName:      arg.Variable, // TODO: normalize this to go-style
		GoType:      typeForInputType(arg.Type, schema),
		// TODO: figure out what to do about defaults
	}
}

func fromASTOperation(op *ast.OperationDefinition, schema *ast.Schema) (operation, error) {
	// TODO: we may have to actually get the precise query text, in case we
	// want to be hashing it or something like that.  Although maybe
	// there's no reasonable way to do that with several queries in one
	// file.
	var builder strings.Builder
	f := formatter.NewFormatter(&builder)
	f.FormatQueryDocument(&ast.QueryDocument{
		Operations: ast.OperationList{op},
		// TODO: handle fragments
	})

	args := make([]argument, len(op.VariableDefinitions))
	for i, arg := range op.VariableDefinitions {
		args[i] = fromASTArg(arg, schema)
	}

	typ, err := typeForOperation(op, schema)
	if err != nil {
		return operation{}, fmt.Errorf("could not compute return-type for query: %v", err)
	}

	return operation{
		Type: op.Operation,
		Name: op.Name,
		// TODO: this is actually awkward, because GraphQL doesn't allow
		// for docstrings on queries (only schemas).  So we have to extract
		// the comment, or omit doc-comments for now.
		Doc: "TODO",
		// The newline just makes it format a little nicer
		Body: "\n" + builder.String(),
		Args: args,

		// TODO: configure ResponseName format
		ResponseName: op.Name + "Response",
		ResponseType: typ,
	}, nil
}

func Generate(config *Config) ([]byte, error) {
	schema, err := getSchema(config.Schema)
	if err != nil {
		return nil, err
	}

	document, err := getAndValidateQueries(config.Queries, schema)
	if err != nil {
		return nil, err
	}

	operations := make([]operation, len(document.Operations))
	for i, op := range document.Operations {
		operations[i], err = fromASTOperation(op, schema)
		if err != nil {
			return nil, err
		}
	}

	data := templateParams{
		// TODO: configure PackageName
		PackageName: "example",
		Operations:  operations,
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return nil, fmt.Errorf("could not render template: %v", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("could not gofmt template: %v", err)
	}

	return formatted, nil
}
