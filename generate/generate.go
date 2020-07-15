package generate

import (
	"bytes"
	"fmt"
	"go/format"
	"path/filepath"
	"runtime"
	"sort"
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

// generator is the context for the codegen process (and ends up getting passed
// to the template).
type generator struct {
	// The config for which we are generating code.
	Config *Config
	// The list of operations for which to generate code.
	Operations []operation
	// The types needed for these operations.
	typeMap   map[string]string
	schema    *ast.Schema
	fragments []*ast.FragmentDefinition
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
}

type argument struct {
	GoName      string
	GoType      string
	GraphQLName string
}

func newGenerator(config *Config, schema *ast.Schema) *generator {
	return &generator{
		Config:  config,
		typeMap: map[string]string{},
		schema:  schema,
	}
}

func (g *generator) Types() string {
	defs := make([]string, 0, len(g.typeMap))
	for _, def := range g.typeMap {
		defs = append(defs, def)
	}
	// Make sure we have a stable order.  (It's somewhat
	// arbitrary but in practice mostly alphabetical.)
	// TODO: ideally we'd do a nice semantic ordering.
	sort.Strings(defs)
	return strings.Join(defs, "\n\n")
}

func (g *generator) getArgument(arg *ast.VariableDefinition) (argument, error) {
	graphQLName := arg.Variable
	firstRest := strings.SplitN(graphQLName, "", 2)
	goName := strings.ToLower(firstRest[0]) + firstRest[1]
	goType, err := g.getTypeForInputType(arg.Type)
	if err != nil {
		return argument{}, err
	}
	return argument{
		GraphQLName: graphQLName,
		GoName:      goName,
		GoType:      goType,
	}, nil
}

func (g *generator) getDocComment(op *ast.OperationDefinition) string {
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

func (g *generator) addOperation(op *ast.OperationDefinition) error {
	// TODO: we may have to actually get the precise query text, in case we
	// want to be hashing it or something like that.  This is a bit tricky
	// because gqlparser's ast doesn't provide node end-position (only
	// token end-position).  (And with fragments it's not even clear what would
	// be right.)  Maybe add as a config option, and allow only if the document
	// has exactly one query?
	var builder strings.Builder
	f := formatter.NewFormatter(&builder)
	f.FormatQueryDocument(&ast.QueryDocument{
		Operations: ast.OperationList{op},
		// TODO(benkraft): Only include relevant fragments.
		Fragments: g.fragments,
	})

	args := make([]argument, len(op.VariableDefinitions))
	for i, arg := range op.VariableDefinitions {
		var err error
		args[i], err = g.getArgument(arg)
		if err != nil {
			return err
		}
	}

	responseName, err := g.getTypeForOperation(op)
	if err != nil {
		return err
	}

	g.Operations = append(g.Operations, operation{
		Type: op.Operation,
		Name: op.Name,
		Doc:  g.getDocComment(op),
		// The newline just makes it format a little nicer
		Body:         "\n" + builder.String(),
		Args:         args,
		ResponseName: responseName,
	})

	return nil
}

func generate(config *Config) (*generator, error) {
	schema, err := getSchema(config.Schema)
	if err != nil {
		return nil, err
	}

	document, err := getAndValidateQueries(config.Queries, schema)
	if err != nil {
		return nil, err
	}

	g := newGenerator(config, schema)
	for _, frag := range document.Fragments {
		if err = g.addFragment(frag); err != nil {
			return nil, err
		}
	}
	for _, op := range document.Operations {
		if err = g.addOperation(op); err != nil {
			return nil, err
		}
	}

	return g, nil
}

func Generate(config *Config) ([]byte, error) {
	g, err := generate(config)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, g)
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
