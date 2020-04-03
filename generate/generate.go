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

func fromASTArg(arg *ast.VariableDefinition, schema *ast.Schema) (argument, error) {
	graphQLName := arg.Variable
	firstRest := strings.SplitN(graphQLName, "", 2)
	goName := strings.ToLower(firstRest[0]) + firstRest[1]
	goType, err := typeForInputType(arg.Type, schema)
	if err != nil {
		return argument{}, err
	}
	return argument{
		GraphQLName: graphQLName,
		GoName:      goName,
		GoType:      goType,
	}, nil
}

func reverse(slice []string) {
	for left, right := 0, len(slice)-1; left < right; left, right = left+1, right-1 {
		slice[left], slice[right] = slice[right], slice[left]
	}
}

func getDocComment(op *ast.OperationDefinition) string {
	var commentLines []string
	var sourceLines = strings.Split(op.Position.Src.Input, "\n")
	for i := op.Position.Line - 1; i > 0; i-- {
		line := sourceLines[i-1]
		if strings.HasPrefix(line, "#") {
			commentLines = append(commentLines,
				"// "+strings.TrimSpace(strings.TrimPrefix(line, "#")))
		} else {
			break
		}
	}

	reverse(commentLines)

	return strings.Join(commentLines, "\n")
}

func fromASTOperation(op *ast.OperationDefinition, schema *ast.Schema) (operation, error) {
	// TODO: we may have to actually get the precise query text, in case we
	// want to be hashing it or something like that.  This is a bit tricky
	// because gqlparser's ast doesn't provide node end-position (only
	// token end-position).
	var builder strings.Builder
	f := formatter.NewFormatter(&builder)
	f.FormatQueryDocument(&ast.QueryDocument{
		Operations: ast.OperationList{op},
		// TODO: handle fragments
	})

	args := make([]argument, len(op.VariableDefinitions))
	for i, arg := range op.VariableDefinitions {
		var err error
		args[i], err = fromASTArg(arg, schema)
		if err != nil {
			return operation{}, err
		}
	}

	typ, err := typeForOperation(op, schema)
	if err != nil {
		return operation{}, fmt.Errorf("could not compute return-type for query: %v", err)
	}

	return operation{
		Type: op.Operation,
		Name: op.Name,
		Doc:  getDocComment(op),
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
		PackageName: config.Package,
		Operations:  operations,
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return nil, fmt.Errorf("could not render template: %v", err)
	}

	unformatted := buf.Bytes()
	formatted, err := format.Source(unformatted)
	if err != nil {
		return nil, fmt.Errorf("could not gofmt code: %v\n---unformatted code---\n%v",
			err, string(unformatted))
	}

	return formatted, nil
}
