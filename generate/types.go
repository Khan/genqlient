package generate

import (
	"fmt"
	"strings"

	"github.com/vektah/gqlparser/ast"
)

type typeBuilder struct {
	strings.Builder
	schema *ast.Schema
}

func typeForOperation(operation *ast.OperationDefinition, schema *ast.Schema) (string, error) {
	builder := &typeBuilder{schema: schema}
	err := builder.writeSelectionSetStruct(operation.SelectionSet)
	return builder.String(), err
}

func typeForInputType(typ *ast.Type, schema *ast.Schema) (string, error) {
	builder := &typeBuilder{schema: schema}

	// TODO: handle non-scalar types (by passing ...something... as the
	// SelectionSet?)
	err := builder.writeType(typ, nil)

	return builder.String(), err
}

func (builder *typeBuilder) writeSelectionSetStruct(selectionSet ast.SelectionSet) error {
	builder.WriteString("struct {\n")
	for _, selection := range selectionSet {
		switch selection := selection.(type) {
		case *ast.Field:
			var jsonName string
			if selection.Alias != "" {
				jsonName = selection.Alias
			} else {
				// TODO: is this case needed? tests don't seem to get here.
				jsonName = selection.Name
			}
			// We need an exportable name for JSON-marshaling.
			goName := strings.Title(jsonName)

			builder.WriteString(goName)
			builder.WriteRune(' ')

			if selection.Definition == nil {
				// Unclear why gqlparser hasn't already rejected this,
				// but empirically it might not.
				return fmt.Errorf("undefined selection %v", selection)
			}
			err := builder.writeType(selection.Definition.Type, selection.SelectionSet)
			if err != nil {
				return err
			}

			if jsonName != goName {
				builder.WriteString("`json:\"")
				builder.WriteString(jsonName)
				builder.WriteString("\"`")
			}
			builder.WriteRune('\n')

		case *ast.FragmentSpread, *ast.InlineFragment:
			return fmt.Errorf("not implemented: %T", selection)
		default:
			return fmt.Errorf("invalid selection type: %v", selection)
		}
	}
	builder.WriteString("}")
	return nil
}

var graphQLNameToGoName = map[string]string{
	"Int":     "int", // TODO: technically int32 is always enough, use that?
	"Float":   "float64",
	"String":  "string",
	"Boolean": "bool",
	"ID":      "string", // TODO: named type for IDs?
}

func (builder *typeBuilder) writeType(typ *ast.Type, selectionSet ast.SelectionSet) error {
	// gqlgen does slightly different things here since it defines names for
	// all the intermediate types, but its implementation may be useful to crib
	// from:
	// https://github.com/99designs/gqlgen/blob/master/plugin/modelgen/models.go#L113
	// TODO: or maybe we should do that?
	if typ.Elem != nil {
		// Type is a list.
		builder.WriteString("[]")
		typ = typ.Elem
	}
	if !typ.NonNull {
		builder.WriteString("*")
	}

	if selectionSet != nil {
		return builder.writeSelectionSetStruct(selectionSet)
	}

	// TODO: handle enums better.  (do unions need special handling?)
	goName := graphQLNameToGoName[typ.Name()]
	if goName == "" {
		return fmt.Errorf("unknown scalar name: %s", typ.Name())
	}
	builder.WriteString(goName)
	return nil
}
