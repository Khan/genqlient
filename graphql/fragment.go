package graphql

import (
	"fmt"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
	"github.com/vektah/gqlparser/v2/validator"
)

// DynFragment is the interface that the generated code calls for make query
// which fragment
type DynFragment interface {
	// ComposeQuery take query
	ComposeQuery(query string) (string, error)
	SetFragment(fragment string)
}

type dynFragment struct {
	schema   *ast.Schema
	fragment string
}

func (d *dynFragment) SetFragment(fragment string) {
	d.fragment = fragment
}

func (d *dynFragment) ComposeQuery(query string) (string, error) {
	queryReturnMerged := fmt.Sprintf("%s\n\n%s", query, d.fragment)
	queryDoc, err := parser.ParseQuery(&ast.Source{
		Name: "noQueryName.graphql", Input: queryReturnMerged,
	})
	if err != nil {
		return "", err
	}

	// Cf. gqlparser.LoadQuery
	graphqlErrors := validator.Validate(d.schema, queryDoc)
	if graphqlErrors != nil {
		return "", fmt.Errorf("query-spec does not match schema: %v", graphqlErrors)
	}

	return queryReturnMerged, nil
}

// NewDynFragment implement DynFragment interface, the parameter is
// Schema and Fragment, this use for
func NewDynFragment(schema string, fragment string) (DynFragment, error) {
	schemaParsed, err := gqlparser.LoadSchema(&ast.Source{Name: "noname.graphql", Input: schema})
	if err != nil {
		return nil, err
	}

	return &dynFragment{
		schema:   schemaParsed,
		fragment: fragment,
	}, nil
}

type dummyFragment struct {
	fragment string
}

func (dummy *dummyFragment) SetFragment(frag string) {
	dummy.fragment = frag
}

func (dummy *dummyFragment) ComposeQuery(query string) (string, error) {
	return fmt.Sprintf("%s\n\n%s", query, dummy.fragment), nil
}

func NewDummyFragment(fragment string) (DynFragment, error) {
	return &dummyFragment{fragment}, nil
}
