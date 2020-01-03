package generate

import (
	"fmt"
	"strings"

	"github.com/vektah/gqlparser/ast"
)

func typeForOperation(operation *ast.OperationDefinition, schema *ast.Schema) string {
	var builder strings.Builder

	writeSelectionSetStruct(&builder, operation.SelectionSet, schema)

	return builder.String()
}

func typeForInputType(typ *ast.Type, schema *ast.Schema) string {
	var builder strings.Builder

	// TODO: handle non-scalar types (by passing ...something... as the
	// SelectionSet?)
	writeType(&builder, typ, nil, schema)

	return builder.String()
}

func writeSelectionSetStruct(builder *strings.Builder, selectionSet ast.SelectionSet, schema *ast.Schema) {
	builder.WriteString("struct {\n")
	for _, selection := range selectionSet {
		switch selection := selection.(type) {
		case *ast.Field:
			// TODO: assert it starts with uppercase (or do automatically if
			// alias is not provided; in this case we may need json tags to
			// avoid munging the query)
			if selection.Alias != "" {
				builder.WriteString(selection.Alias)
			} else {
				builder.WriteString(selection.Name)
			}
			builder.WriteRune(' ')

			writeType(builder, selection.Definition.Type, selection.SelectionSet, schema)

			// We don't need a json tag -- we just have GraphQL do the
			// aliasing.
			builder.WriteRune('\n')

		case *ast.FragmentSpread, *ast.InlineFragment:
			panic("TODO")
		default:
			panic(fmt.Errorf("invalid selection type: %v", selection))
		}
	}
	builder.WriteString("}")
}

func writeType(builder *strings.Builder, typ *ast.Type, selectionSet ast.SelectionSet, schema *ast.Schema) {
	if typ.Elem != nil {
		// Type is a list.
		builder.WriteString("[]")
		typ = typ.Elem
	} else if !typ.NonNull { // no need for pointer if we have a list
		builder.WriteString("*")
	}

	if selectionSet != nil {
		writeSelectionSetStruct(builder, selectionSet, schema)
		return
	}

	// TODO: actually handle scalars.  or can we instead use gqlgen's
	// converter?  they're doing mostly the same thing.  if not, crib from it:
	// https://github.com/99designs/gqlgen/blob/master/plugin/modelgen/models.go#L113
	builder.WriteString(strings.ToLower(typ.Name()))
}
