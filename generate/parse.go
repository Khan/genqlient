package generate

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
	"github.com/vektah/gqlparser/v2/validator"
)

func getSchema(filename string) (*ast.Schema, error) {
	text, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("unreadable schema file %v: %v", filename, err)
	}

	schema, graphqlError := gqlparser.LoadSchema(
		&ast.Source{Name: filename, Input: string(text)})
	if graphqlError != nil {
		return nil, fmt.Errorf("invalid schema file %v: %v",
			filename, graphqlError)
	}

	return schema, nil
}

func getAndValidateQueries(filenames []string, schema *ast.Schema) (*ast.QueryDocument, error) {
	// We merge all the queries into a single query-document, since operations
	// in one might reference fragments in another.
	//
	// TODO(benkraft): It might be better to merge just within a filename, so
	// that fragment-names don't need to be unique across files.
	mergedQueryDoc := new(ast.QueryDocument)

	for _, filename := range filenames {
		switch filepath.Ext(filename) {
		case ".graphql":
			// Cf. gqlparser.LoadQuery
			text, err := ioutil.ReadFile(filename)
			if err != nil {
				return nil, fmt.Errorf("unreadable query-spec file %v: %v", filename, err)
			}

			queryDoc, err := getQueriesFromString(string(text), filename)
			if err != nil {
				return nil, err
			}

			mergedQueryDoc.Operations = append(mergedQueryDoc.Operations, queryDoc.Operations...)
			mergedQueryDoc.Fragments = append(mergedQueryDoc.Fragments, queryDoc.Fragments...)

		default:
			return nil, fmt.Errorf("unknown file type: %v", filename)
		}
	}

	graphqlErrors := validator.Validate(schema, mergedQueryDoc)
	if graphqlErrors != nil {
		return nil, fmt.Errorf("query-spec does not match schema: %v", graphqlErrors)
	}

	return mergedQueryDoc, nil
}

func getQueriesFromString(text string, filename string) (*ast.QueryDocument, error) {
	document, graphqlError := parser.ParseQuery(
		&ast.Source{Name: filename, Input: text})
	if graphqlError != nil { // ParseQuery returns type *graphql.Error, yuck
		return nil, fmt.Errorf("invalid query-spec file %v: %v", filename, graphqlError)
	}

	return document, nil
}
