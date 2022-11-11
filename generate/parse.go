package generate

import (
	"fmt"
	goAst "go/ast"
	goParser "go/parser"
	goToken "go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
	"github.com/vektah/gqlparser/v2/validator"
	_ "github.com/vektah/gqlparser/v2/validator/rules"
)

func getSchema(globs StringList) (*ast.Schema, error) {
	filenames, err := expandFilenames(globs)
	if err != nil {
		return nil, err
	}

	sources := make([]*ast.Source, len(filenames))
	for i, filename := range filenames {
		text, err := os.ReadFile(filename)
		if err != nil {
			return nil, errorf(nil, "unreadable schema file %v: %v", filename, err)
		}
		sources[i] = &ast.Source{Name: filename, Input: string(text)}
	}

	// Ideally here we'd just call gqlparser.LoadSchema. But the schema we are
	// given may or may not contain the builtin types String, Int, etc. (The
	// spec says it shouldn't, but introspection will return those types, and
	// some introspection-to-SDL tools aren't smart enough to remove them.) So
	// we inline LoadSchema and insert some checks.
	document, graphqlError := parser.ParseSchemas(sources...)
	if graphqlError != nil {
		// Schema doesn't even parse.
		return nil, errorf(nil, "invalid schema: %v", graphqlError)
	}

	// Check if we have a builtin type. (String is an arbitrary choice.)
	hasBuiltins := false
	for _, def := range document.Definitions {
		if def.Name == "String" {
			hasBuiltins = true
			break
		}
	}

	if !hasBuiltins {
		// modified from parser.ParseSchemas
		var preludeAST *ast.SchemaDocument
		preludeAST, graphqlError = parser.ParseSchema(validator.Prelude)
		if graphqlError != nil {
			return nil, errorf(nil, "invalid prelude (probably a gqlparser bug): %v", graphqlError)
		}
		document.Merge(preludeAST)
	}

	schema, graphqlError := validator.ValidateSchemaDocument(document)
	if graphqlError != nil {
		return nil, errorf(nil, "invalid schema: %v", graphqlError)
	}

	return schema, nil
}

// TODO document, note take care bc document may be wildly invalid
// TODO: consider merging preprocessQueryDocument into here (can't put __all
// there bc needs to happen before validation)
// TODO: document __all, looks roughly like __all(depth: Int = 1)
func mungeQueries(schema *ast.Schema, queryDoc *ast.QueryDocument) error {
	var observers validator.Events
	handleAllField := func(field *ast.Field, parentDef *ast.Definition) ast.SelectionSet {
		if field.Alias != "__all" {
			panic("can't have alias for __all")
		}

		depth := 1
		seenDepth := false
		for _, argument := range field.Arguments {
			if argument.Name != "depth" || seenDepth {
				panic("must have at most one argument, called depth")
			}
			// TODO: also ban directives, selections
			seenDepth = true
			// TODO: be explicit about handling of variables
			v, err := argument.Value.Value(nil)
			if err != nil {
				panic(err)
			}
			depth = int(v.(int64))
			if depth <= 0 {
				panic("depth must be positive")
			}
		}

		if depth != 1 {
			panic("TODO: implement depth != 1")
		}

		var replacements ast.SelectionSet
		for _, fieldDef := range parentDef.Fields {
			fieldTyp := schema.Types[fieldDef.Type.Name()]
			if fieldTyp.Kind != ast.Scalar && fieldTyp.Kind != ast.Enum {
				continue // not a leaf, TODO recurse for depth
			}
			replacements = append(replacements, &ast.Field{
				Alias: fieldDef.Name,
				Name:  fieldDef.Name,
				// TODO: what if it has required args? skip?
			})
		}

		return replacements
	}

	handleSelectionSet := func(selectionSet ast.SelectionSet, parentDef *ast.Definition) ast.SelectionSet {
		for i := 0; i < len(selectionSet); {
			field, ok := selectionSet[i].(*ast.Field)
			if !ok || field.Name != "__all" {
				i++
				continue
			}
			replacements := handleAllField(field, parentDef)
			selectionSet = append(append(append(
				// TODO: update in-place? nontrivial to do safely
				ast.SelectionSet{},
				selectionSet[:i]...),
				replacements...),
				selectionSet[i+1:]...)
			i += len(replacements)
		}
		return selectionSet
	}

	// TODO: also wire up to OnInlineFragment, OnFragmentSpread,
	// OnOperation
	observers.OnField(func(_ *validator.Walker, field *ast.Field) {
		if field.Definition == nil || field.Definition.Type == nil {
			return
		}
		field.SelectionSet = handleSelectionSet(field.SelectionSet, schema.Types[field.Definition.Type.Name()])
	})

	validator.Walk(schema, queryDoc, &observers)
	return nil
}

func getAndValidateQueries(basedir string, filenames StringList, schema *ast.Schema) (*ast.QueryDocument, error) {
	queryDoc, err := getQueries(basedir, filenames)
	if err != nil {
		return nil, err
	}

	err = mungeQueries(schema, queryDoc)
	if err != nil {
		return nil, err
	}

	// Cf. gqlparser.LoadQuery
	graphqlErrors := validator.Validate(schema, queryDoc)
	if graphqlErrors != nil {
		return nil, errorf(nil, "query-spec does not match schema: %v", graphqlErrors)
	}

	return queryDoc, nil
}

func expandFilenames(globs []string) ([]string, error) {
	uniqFilenames := make(map[string]bool, len(globs))
	for _, glob := range globs {
		matches, err := filepath.Glob(glob)
		if err != nil {
			return nil, errorf(nil, "can't expand file-glob %v: %v", glob, err)
		}
		if len(matches) == 0 {
			return nil, errorf(nil, "%v did not match any files", glob)
		}
		for _, match := range matches {
			uniqFilenames[match] = true
		}
	}
	filenames := make([]string, 0, len(uniqFilenames))
	for filename := range uniqFilenames {
		filenames = append(filenames, filename)
	}
	return filenames, nil
}

func getQueries(basedir string, globs StringList) (*ast.QueryDocument, error) {
	// We merge all the queries into a single query-document, since operations
	// in one might reference fragments in another.
	//
	// TODO(benkraft): It might be better to merge just within a filename, so
	// that fragment-names don't need to be unique across files.  (Although
	// then we may have other problems; and query-names still need to be.)
	mergedQueryDoc := new(ast.QueryDocument)
	addQueryDoc := func(queryDoc *ast.QueryDocument) {
		mergedQueryDoc.Operations = append(mergedQueryDoc.Operations, queryDoc.Operations...)
		mergedQueryDoc.Fragments = append(mergedQueryDoc.Fragments, queryDoc.Fragments...)
	}

	filenames, err := expandFilenames(globs)
	if err != nil {
		return nil, err
	}

	for _, filename := range filenames {
		text, err := os.ReadFile(filename)
		if err != nil {
			return nil, errorf(nil, "unreadable query-spec file %v: %v", filename, err)
		}

		switch filepath.Ext(filename) {
		case ".graphql":
			queryDoc, err := getQueriesFromString(string(text), basedir, filename)
			if err != nil {
				return nil, err
			}

			addQueryDoc(queryDoc)

		case ".go":
			queryDocs, err := getQueriesFromGo(string(text), basedir, filename)
			if err != nil {
				return nil, err
			}

			for _, queryDoc := range queryDocs {
				addQueryDoc(queryDoc)
			}

		default:
			return nil, errorf(nil, "unknown file type: %v", filename)
		}
	}

	return mergedQueryDoc, nil
}

func getQueriesFromString(text string, basedir, filename string) (*ast.QueryDocument, error) {
	// make path relative to the config-directory
	relname, err := filepath.Rel(basedir, filename)
	if err == nil {
		filename = relname
	}

	// Cf. gqlparser.LoadQuery
	document, graphqlError := parser.ParseQuery(
		&ast.Source{Name: filename, Input: text})
	if graphqlError != nil { // ParseQuery returns type *graphql.Error, yuck
		return nil, errorf(nil, "invalid query-spec file %v: %v", filename, graphqlError)
	}

	return document, nil
}

func getQueriesFromGo(text string, basedir, filename string) ([]*ast.QueryDocument, error) {
	fset := goToken.NewFileSet()
	f, err := goParser.ParseFile(fset, filename, text, 0)
	if err != nil {
		return nil, errorf(nil, "invalid Go file %v: %v", filename, err)
	}

	var retval []*ast.QueryDocument
	goAst.Inspect(f, func(node goAst.Node) bool {
		if err != nil {
			return false // don't bother to recurse if something already failed
		}

		basicLit, ok := node.(*goAst.BasicLit)
		if !ok || basicLit.Kind != goToken.STRING {
			return true // recurse
		}

		var value string
		value, err = strconv.Unquote(basicLit.Value)
		if err != nil {
			return false
		}

		if !strings.HasPrefix(strings.TrimSpace(value), "# @genqlient") {
			return true
		}

		// We put the filename as <real filename>:<line>, which errors.go knows
		// how to parse back out (since it's what gqlparser will give to us in
		// our errors).
		pos := fset.Position(basicLit.Pos())
		fakeFilename := fmt.Sprintf("%v:%v", pos.Filename, pos.Line)
		var query *ast.QueryDocument
		query, err = getQueriesFromString(value, basedir, fakeFilename)
		if err != nil {
			return false
		}
		retval = append(retval, query)

		return true
	})

	return retval, err
}
