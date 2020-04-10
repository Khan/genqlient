package generate

import (
	"fmt"
	"strings"

	"github.com/vektah/gqlparser/ast"
)

type typeBuilder struct {
	strings.Builder
	*generator
}

func (g *generator) baseTypeForOperation(operation ast.Operation) *ast.Definition {
	switch operation {
	case ast.Query:
		return g.schema.Query
	case ast.Mutation:
		return g.schema.Mutation
	case ast.Subscription:
		return g.schema.Subscription
	default:
		panic(fmt.Sprintf("unexpected operation: %v", operation))
	}
}

func (g *generator) addTypeForOperation(operation *ast.OperationDefinition) (name string, err error) {
	// TODO: configure ResponseName format
	name = operation.Name + "Response"

	if def, ok := g.typeMap[name]; ok {
		// TODO: if the name is taken, maybe try to find another?
		return "", fmt.Errorf("%s already defined:\n%s", name, def)
	}

	builder := &typeBuilder{generator: g}
	fmt.Fprintf(builder, "type %s ", name)
	err = builder.writeTypedef(
		g.baseTypeForOperation(operation.Operation), operation.SelectionSet)
	if err != nil {
		return "", err
	}

	def := builder.String()
	g.typeMap[name] = def
	return name, nil
}

func (g *generator) addTypeForInputType(typ *ast.Type) (string, error) {
	builder := &typeBuilder{generator: g}

	// TODO: handle non-scalar types (by passing ...something... as the
	// SelectionSet?)
	err := builder.writeType(typ, nil)

	return builder.String(), err
}

func (builder *typeBuilder) writeField(field *ast.Field) error {
	var jsonName string
	if field.Alias != "" {
		jsonName = field.Alias
	} else {
		// TODO: is this case needed? tests don't seem to get here.
		jsonName = field.Name
	}
	// We need an exportable name for JSON-marshaling.
	goName := strings.Title(jsonName)

	builder.WriteString(goName)
	builder.WriteRune(' ')

	if field.Definition == nil {
		// Unclear why gqlparser hasn't already rejected this,
		// but empirically it might not.
		return fmt.Errorf("undefined field %v", field)
	}
	err := builder.writeType(field.Definition.Type, field.SelectionSet)
	if err != nil {
		return err
	}

	if jsonName != goName {
		builder.WriteString("`json:\"")
		builder.WriteString(jsonName)
		builder.WriteString("\"`")
	}
	builder.WriteRune('\n')
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

	return builder.writeTypedef(builder.schema.Types[typ.Name()], selectionSet)
}

func (builder *typeBuilder) writeTypedef(typedef *ast.Definition, selectionSet ast.SelectionSet) error {
	switch typedef.Kind {
	case ast.Object, ast.InputObject:
		builder.WriteString("struct {\n")
		for _, selection := range selectionSet {
			switch selection := selection.(type) {
			case *ast.Field:
				builder.writeField(selection)
			case *ast.FragmentSpread, *ast.InlineFragment:
				return fmt.Errorf("not implemented: %T", selection)
			default:
				return fmt.Errorf("invalid selection type: %v", selection)
			}
		}
		builder.WriteString("}")
		return nil
	case ast.Scalar, ast.Enum:
		goName := graphQLNameToGoName[typedef.Name]
		// TODO(benkraft): Handle custom scalars and enums.
		if goName == "" {
			return fmt.Errorf("unknown scalar name: %s", typedef.Name)
		}
		builder.WriteString(goName)
		return nil
	case ast.Union, ast.Interface:
		return fmt.Errorf("not implemented: %v", typedef.Kind)
	default:
		return fmt.Errorf("unexpected kind: %v", typedef.Kind)
	}
}
