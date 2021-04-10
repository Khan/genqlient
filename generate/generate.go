package generate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"sort"
	"strings"
	"text/template"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/formatter"
	"golang.org/x/tools/imports"
)

// Set to true to test features that aren't yet really ready.
var allowBrokenFeatures = false

// generator is the context for the codegen process (and ends up getting passed
// to the template).
type generator struct {
	// The config for which we are generating code.
	Config *Config
	// The list of operations for which to generate code.
	Operations []operation
	// The types needed for these operations.
	typeMap map[string]string
	// Imports needed for these operations, path -> alias and alias -> true
	imports     map[string]string
	usedAliases map[string]bool
	// Cache of loaded templates.
	templateCache map[string]*template.Template
	// Schema we are generating code against
	schema *ast.Schema
}

// JSON tags in operation are for ExportOperations (see Config for details).
type operation struct {
	// The type of the operation (query, mutation, or subscription).
	Type ast.Operation `json:"-"`
	// The name of the operation, from GraphQL.
	Name string `json:"operationName"`
	// The documentation for the operation, from GraphQL.
	Doc string `json:"-"`
	// The body of the operation to send.
	Body string `json:"query"`
	// The arguments to the operation.
	Args []argument `json:"-"`
	// The type-name for the operation's response type.
	ResponseName string `json:"-"`
	// The original location of this query.
	SourceLocation string `json:"sourceLocation"`
}

type exportedOperations struct {
	Operations []operation `json:"operations"`
}

type argument struct {
	GoName      string
	GoType      string
	GraphQLName string
	Options     *GenqlientDirective
}

func newGenerator(config *Config, schema *ast.Schema) *generator {
	g := generator{
		Config:        config,
		typeMap:       map[string]string{},
		imports:       map[string]string{},
		usedAliases:   map[string]bool{},
		templateCache: map[string]*template.Template{},
		schema:        schema,
	}

	if g.Config.ClientGetter == "" {
		_, err := g.addRef("github.com/Khan/genqlient/graphql.Client")
		if err != nil {
			panic(err)
		}
	}
	if g.Config.ContextType != "" {
		_, err := g.addRef(g.Config.ContextType)
		if err != nil {
			panic(err)
		}
	}

	return &g
}

func (g *generator) Types() string {
	names := make([]string, 0, len(g.typeMap))
	for name := range g.typeMap {
		names = append(names, name)
	}
	// Sort alphabetically by type-name.  Sorting somehow deterministically is
	// important to ensure generated code is deterministic.  Alphabetical is
	// nice because it's easy, and in the current naming scheme, it's even
	// vaguely aligned to the structure of the queries.
	sort.Strings(names)

	defs := make([]string, 0, len(g.typeMap))
	for _, name := range names {
		defs = append(defs, g.typeMap[name])
	}
	return strings.Join(defs, "\n\n")
}

func (g *generator) getArgument(
	opName string,
	arg *ast.VariableDefinition,
	operationDirective *GenqlientDirective,
) (argument, error) {
	_, directive, err := g.parsePrecedingComment(arg, arg.Position)
	if err != nil {
		return argument{}, err
	}

	graphQLName := arg.Variable
	goType, err := g.getTypeForInputType(
		opName, arg.Type, directive, operationDirective)
	if err != nil {
		return argument{}, err
	}
	return argument{
		GraphQLName: graphQLName,
		GoName:      lowerFirst(graphQLName),
		GoType:      goType,
		Options:     directive,
	}, nil
}

func (g *generator) addOperation(op *ast.OperationDefinition) error {
	if op.Name == "" {
		return fmt.Errorf("operations must have operation-names")
	}

	var builder strings.Builder
	f := formatter.NewFormatter(&builder)
	// TODO: this could even get minifed.
	f.FormatQueryDocument(&ast.QueryDocument{
		Operations: ast.OperationList{op},
		// TODO: handle fragments
	})

	commentLines, directive, err := g.parsePrecedingComment(op, op.Position)
	if err != nil {
		return err
	}

	args := make([]argument, len(op.VariableDefinitions))
	for i, arg := range op.VariableDefinitions {
		var err error
		args[i], err = g.getArgument(op.Name, arg, directive)
		if err != nil {
			return err
		}
	}

	responseName, err := g.getTypeForOperation(op, directive)
	if err != nil {
		return err
	}

	var docComment string
	if len(commentLines) > 0 {
		docComment = "// " + strings.ReplaceAll(commentLines, "\n", "\n// ")
	}

	g.Operations = append(g.Operations, operation{
		Type: op.Operation,
		Name: op.Name,
		Doc:  docComment,
		// The newline just makes it format a little nicer.
		Body:           "\n" + builder.String(),
		Args:           args,
		ResponseName:   responseName,
		SourceLocation: op.Position.Src.Name,
	})

	return nil
}

// Generate returns a map from absolute-path filename to generated content.
func Generate(config *Config) (map[string][]byte, error) {
	schema, err := getSchema(config.Schema)
	if err != nil {
		return nil, err
	}

	document, err := getAndValidateQueries(config.baseDir(), config.Operations, schema)
	if err != nil {
		return nil, err
	}

	// TODO: we could also allow this, and generate an empty file with just the
	// package-name, if it turns out to be more convenient that way.  (As-is,
	// we generate a broken file, with just (unused) imports.)
	if len(document.Operations) == 0 {
		return nil, fmt.Errorf("no queries found in %v", config.Operations)
	}

	if len(document.Fragments) > 0 && !allowBrokenFeatures {
		return nil, fmt.Errorf("genqlient does not yet support fragments")
	}

	g := newGenerator(config, schema)
	for _, op := range document.Operations {
		if err = g.addOperation(op); err != nil {
			return nil, err
		}
	}

	var buf bytes.Buffer
	err = g.execute("operation.go.tmpl", &buf, g)
	if err != nil {
		return nil, fmt.Errorf("could not render template: %v", err)
	}

	unformatted := buf.Bytes()
	formatted, err := format.Source(unformatted)
	if err != nil {
		return nil, fmt.Errorf("could not gofmt code: %v\n---unformatted code---\n%v",
			err, string(unformatted))
	}
	importsed, err := imports.Process(config.Generated, formatted, nil)
	if err != nil {
		return nil, fmt.Errorf("could not goimports code: %v\n---unimportsed code---\n%v",
			err, string(formatted))
	}

	retval := map[string][]byte{
		config.Generated: importsed,
	}

	if config.ExportOperations != "" {
		// We use MarshalIndent so that the file is human-readable and
		// slightly more likely to be git-mergeable (if you check it in).  In
		// general it's never going to be used anywhere where space is an
		// issue -- it doesn't go in your binary or anything.
		retval[config.ExportOperations], err = json.MarshalIndent(
			exportedOperations{Operations: g.Operations}, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("unable to export queries: %v", err)
		}
	}

	return retval, nil
}
