package generate

// This file implements the main entrypoint and framework for the genqlient
// code-generation process.  See comments in Generate for the high-level
// overview.

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
	"github.com/vektah/gqlparser/v2/validator"
	"golang.org/x/tools/imports"
)

// generator is the context for the codegen process (and ends up getting passed
// to the template).
type generator struct {
	// The config for which we are generating code.
	Config *Config
	// The list of operations for which to generate code.
	Operations []operation
	// The types needed for these operations.
	typeMap map[string]goType
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
	// The original filename from which we got this query.
	SourceFilename string `json:"sourceLocation"`
}

type exportedOperations struct {
	Operations []operation `json:"operations"`
}

type argument struct {
	GoName      string
	GoType      string
	GraphQLName string
	IsSlice     bool
	Options     *GenqlientDirective
}

func newGenerator(config *Config, schema *ast.Schema) (*generator, error) {
	g := generator{
		Config:        config,
		typeMap:       map[string]goType{},
		imports:       map[string]string{},
		usedAliases:   map[string]bool{},
		templateCache: map[string]*template.Template{},
		schema:        schema,
	}

	_, err := g.addRef("github.com/Khan/genqlient/graphql.Client")
	if err != nil {
		return nil, err
	}

	if g.Config.ClientGetter != "" {
		_, err := g.addRef(g.Config.ClientGetter)
		if err != nil {
			return nil, fmt.Errorf("invalid client_getter: %w", err)
		}
	}

	if g.Config.ContextType != "" {
		_, err := g.addRef(g.Config.ContextType)
		if err != nil {
			return nil, fmt.Errorf("invalid context_type: %w", err)
		}
	}

	return &g, nil
}

func (g *generator) Types() (string, error) {
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
	var builder strings.Builder
	for _, name := range names {
		builder.Reset()
		err := g.typeMap[name].WriteDefinition(&builder, g)
		if err != nil {
			return "", err
		}
		defs = append(defs, builder.String())
	}
	return strings.Join(defs, "\n\n"), nil
}

func (g *generator) getArgument(
	arg *ast.VariableDefinition,
	operationDirective *GenqlientDirective,
) (argument, error) {
	_, directive, err := g.parsePrecedingComment(arg, arg.Position)
	if err != nil {
		return argument{}, err
	}

	graphQLName := arg.Variable
	goTyp, err := g.convertInputType(arg.Type, directive, operationDirective)
	if err != nil {
		return argument{}, err
	}
	return argument{
		GraphQLName: graphQLName,
		GoName:      lowerFirst(graphQLName),
		GoType:      goTyp.Reference(),
		IsSlice:     arg.Type.Elem != nil,
		Options:     operationDirective.merge(directive),
	}, nil
}

// Preprocess each query to make any changes that genqlient needs.
//
// At present, the only change is that we add __typename, if not already
// requested, to each field of interface type, so we can use the right types
// when unmarshaling.
func (g *generator) preprocessQueryDocument(doc *ast.QueryDocument) {
	var observers validator.Events
	// We want to ensure that everywhere you ask for some list of fields (a
	// selection-set) from an interface (or union) type, you ask for its
	// __typename field.  There are four places we might find a selection-set:
	// at the toplevel of a query, on a field, or in an inline or named
	// fragment.  The toplevel of a query must be an object type, so we don't
	// need to consider that.
	// TODO(benkraft): Once we support fragments, figure out whether we need to
	// traverse inline/named fragments here too.
	observers.OnField(func(_ *validator.Walker, field *ast.Field) {
		// We are interested in a field from the query like
		//	field { subField ... }
		// where the schema looks like
		//	type ... {       # or interface/union
		//		field: FieldType    # or [FieldType!]! etc.
		//	}
		//	interface FieldType {   # or union
		//		subField: ...
		//	}
		// If FieldType is an interface/union, and none of the subFields is
		// __typename, we want to change the query to
		//	field { __typename subField ... }

		fieldType := g.schema.Types[field.Definition.Type.Name()]
		if fieldType.Kind != ast.Interface && fieldType.Kind != ast.Union {
			return // a concrete type
		}

		hasTypename := false
		for _, selection := range field.SelectionSet {
			// Check if we already selected __typename. We ignore fragments,
			// because we want __typename as a toplevel field.
			subField, ok := selection.(*ast.Field)
			if ok && subField.Name == "__typename" {
				hasTypename = true
			}
		}
		if !hasTypename {
			// Ok, we need to add the field!
			field.SelectionSet = append(ast.SelectionSet{
				&ast.Field{
					Alias: "__typename", Name: "__typename",
					// Fake definition for the magic field __typename cribbed
					// from gqlparser's validator/walk.go, equivalent to
					//	__typename: String
					// TODO(benkraft): This should in principle be
					//	__typename: String!
					// But genqlient doesn't care, so we just match gqlparser.
					Definition: &ast.FieldDefinition{
						Name: "__typename",
						Type: ast.NamedType("String", nil /* pos */),
					},
					// Definition of the object that contains this field, i.e.
					// FieldType.
					ObjectDefinition: fieldType,
				},
			}, field.SelectionSet...)
		}
	})
	validator.Walk(g.schema, doc, &observers)
}

func (g *generator) addOperation(op *ast.OperationDefinition) error {
	if op.Name == "" {
		return errorf(op.Position, "operations must have operation-names")
	}

	queryDoc := &ast.QueryDocument{
		Operations: ast.OperationList{op},
		// TODO: handle fragments
	}
	g.preprocessQueryDocument(queryDoc)

	var builder strings.Builder
	f := formatter.NewFormatter(&builder)
	f.FormatQueryDocument(queryDoc)

	commentLines, directive, err := g.parsePrecedingComment(op, op.Position)
	if err != nil {
		return err
	}

	args := make([]argument, len(op.VariableDefinitions))
	for i, arg := range op.VariableDefinitions {
		args[i], err = g.getArgument(arg, directive)
		if err != nil {
			return err
		}
	}

	responseType, err := g.convertOperation(op, directive)
	if err != nil {
		return err
	}

	var docComment string
	if len(commentLines) > 0 {
		docComment = "// " + strings.ReplaceAll(commentLines, "\n", "\n// ")
	}

	// If the filename is a pseudo-filename filename.go:startline, just
	// put the filename in the export; we don't figure out the line offset
	// anyway, and if you want to check those exports in they will change a
	// lot if they have line numbers.
	// TODO: refactor to use the errorPos machinery for this
	sourceFilename := op.Position.Src.Name
	if i := strings.LastIndex(sourceFilename, ":"); i != -1 {
		sourceFilename = sourceFilename[:i]
	}

	g.Operations = append(g.Operations, operation{
		Type: op.Operation,
		Name: op.Name,
		Doc:  docComment,
		// The newline just makes it format a little nicer.
		Body:           "\n" + builder.String(),
		Args:           args,
		ResponseName:   responseType.Reference(),
		SourceFilename: sourceFilename,
	})

	return nil
}

// Generate returns a map from absolute-path filename to generated content.
//
// This is the main entrypoint to the code-generation process for callers who
// wish to manage the config-reading (ReadAndValidateConfig) and file-writing
// on their own.  (Those are wired in by Main.)
func Generate(config *Config) (map[string][]byte, error) {
	// Step 1: Read in the schema and operations from the files defined by the
	// config (and validate the operations against the schema).  This is all
	// defined in parse.go.
	schema, err := getSchema(config.Schema)
	if err != nil {
		return nil, err
	}

	document, err := getAndValidateQueries(config.baseDir(), config.Operations, schema)
	if err != nil {
		return nil, err
	}

	// TODO(benkraft): we could also allow this, and generate an empty file
	// with just the package-name, if it turns out to be more convenient that
	// way.  (As-is, we generate a broken file, with just (unused) imports.)
	if len(document.Operations) == 0 {
		// Hard to have a position when there are no operations :(
		return nil, errorf(nil, "no queries found, looked in: %v",
			strings.Join(config.Operations, ", "))
	}

	if len(document.Fragments) > 0 && !config.AllowBrokenFeatures {
		return nil, errorf(document.Fragments[0].Position,
			"genqlient does not yet support fragments")
	}

	// Step 2: For each operation, convert it into data structures representing
	// Go types (defined in types.go).  The bulk of this logic is in
	// convert.go.
	g, err := newGenerator(config, schema)
	if err != nil {
		return nil, err
	}

	for _, op := range document.Operations {
		if err = g.addOperation(op); err != nil {
			return nil, err
		}
	}

	// Step 3: Glue it all together!  Most of this is done inline in the
	// template, but the call to g.Types() in the template calls out to
	// types.go to actually generate the code for each type.
	var buf bytes.Buffer
	err = g.execute("operation.go.tmpl", &buf, g)
	if err != nil {
		return nil, errorf(nil, "could not render template: %v", err)
	}

	unformatted := buf.Bytes()
	formatted, err := format.Source(unformatted)
	if err != nil {
		return nil, errorf(nil, "could not gofmt code: %v\n---unformatted code---\n%v",
			err, string(unformatted))
	}
	importsed, err := imports.Process(config.Generated, formatted, nil)
	if err != nil {
		return nil, errorf(nil, "could not goimports code: %v\n---unimportsed code---\n%v",
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
			return nil, errorf(nil, "unable to export queries: %v", err)
		}
	}

	return retval, nil
}
