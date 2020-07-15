package generate

import (
	"fmt"
	"strings"

	"github.com/vektah/gqlparser/ast"
)

type typeBuilder struct {
	typeName string
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

func (g *generator) getTypeForOperation(operation *ast.OperationDefinition) (name string, err error) {
	// TODO: configure ResponseName format
	name = operation.Name + "Response"

	if def, ok := g.typeMap[name]; ok {
		// TODO: check for and handle conflicts a better way
		return "", fmt.Errorf("%s already defined:\n%s", name, def)
	}

	selectionSet, err := selections(operation.SelectionSet)
	if err != nil {
		return "", err
	}

	return g.addTypeForDefinition(
		name, g.baseTypeForOperation(operation.Operation), selectionSet)
}

func (g *generator) addTypeForDefinition(nameOverride string, typ *ast.Definition, selectionSet []selection) (name string, err error) {
	goName, ok := builtinTypes[typ.Name]
	if ok {
		return goName, nil
	}

	if nameOverride != "" {
		name = nameOverride
	} else {
		// TODO: casing should be configurable
		name = lowerFirst(typ.Name)
	}

	if _, ok := g.typeMap[name]; ok {
		return name, nil
	}

	builder := &typeBuilder{typeName: name, generator: g}
	fmt.Fprintf(builder, "type %s ", name)
	err = builder.writeTypedef(typ, selectionSet)
	if err != nil {
		return "", err
	}

	g.typeMap[name] = builder.String()
	return name, nil
}

func (g *generator) getTypeForInputType(typ *ast.Type) (string, error) {
	builder := &typeBuilder{typeName: lowerFirst(typ.Name()), generator: g}
	err := builder.writeType(typ, selectionsForType(g, typ), false)
	return builder.String(), err
}

type selection interface {
	Alias() string
	Name() string
	Type() *ast.Type
	SelectionSet() ([]selection, error)
}

type field struct{ field *ast.Field }

func (s field) Alias() string { return s.field.Alias }
func (s field) Name() string  { return s.field.Name }

func (s field) Type() *ast.Type {
	if s.field.Definition == nil {
		return nil
	}
	return s.field.Definition.Type
}

func (s field) SelectionSet() ([]selection, error) {
	return selections(s.field.SelectionSet)
}

func selections(selectionSet ast.SelectionSet) ([]selection, error) {
	retval := make([]selection, len(selectionSet))
	for i, selection := range selectionSet {
		switch selection := selection.(type) {
		case *ast.Field:
			retval[i] = field{selection}
		case *ast.FragmentSpread, *ast.InlineFragment:
			return nil, fmt.Errorf("not implemented: %T", selection)
		default:
			return nil, fmt.Errorf("invalid selection type: %v", selection)
		}
	}
	return retval, nil
}

type inputField struct {
	*generator
	field *ast.FieldDefinition
}

func (s inputField) Alias() string   { return s.field.Name }
func (s inputField) Name() string    { return s.field.Name }
func (s inputField) Type() *ast.Type { return s.field.Type }

func (s inputField) SelectionSet() ([]selection, error) {
	return selectionsForType(s.generator, s.field.Type), nil
}

func selectionsForType(g *generator, typ *ast.Type) []selection {
	def := g.schema.Types[typ.Name()]
	selectionSet := make([]selection, len(def.Fields))
	for i, field := range def.Fields {
		selectionSet[i] = inputField{g, field}
	}
	return selectionSet
}

func (builder *typeBuilder) writeField(selection selection) error {
	var jsonName string
	if selection.Alias() != "" {
		jsonName = selection.Alias()
	} else {
		// TODO: is this case needed? tests don't seem to get here.
		jsonName = selection.Name()
	}
	// We need an exportable name for JSON-marshaling.
	goName := upperFirst(jsonName)

	builder.WriteString(goName)
	builder.WriteRune(' ')

	typ := selection.Type()
	if typ == nil {
		// Unclear why gqlparser hasn't already rejected this,
		// but empirically it might not.
		return fmt.Errorf("undefined field %v", selection.Name())
	}

	selectionSet, err := selection.SelectionSet()
	if err != nil {
		return err
	}

	err = builder.writeType(typ, selectionSet, true)
	if err != nil {
		return err
	}

	if jsonName != goName {
		fmt.Fprintf(builder, " `json:\"%s\"`", jsonName)
	}
	builder.WriteRune('\n')
	return nil
}

var builtinTypes = map[string]string{
	"Int":     "int", // TODO: technically int32 is always enough, use that?
	"Float":   "float64",
	"String":  "string",
	"Boolean": "bool",
	"ID":      "string", // TODO: named type for IDs?
}

func (builder *typeBuilder) writeType(typ *ast.Type, selectionSet []selection, inline bool) error {
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

	def := builder.schema.Types[typ.Name()]
	switch def.Kind {
	case ast.Scalar, ast.Enum:
		inline = false
	}

	if inline {
		return builder.writeTypedef(def, selectionSet)
	}

	// Writes a typedef elsewhere (if not already defined)
	name, err := builder.addTypeForDefinition("", def, selectionSet)
	if err != nil {
		return err
	}

	builder.WriteString(name)
	return nil
}

func (builder *typeBuilder) writeTypedef(typedef *ast.Definition, selectionSet []selection) error {
	switch typedef.Kind {
	case ast.Object, ast.InputObject, ast.Interface, ast.Union:
		builder.WriteString("struct {\n")
		for _, field := range selectionSet {
			err := builder.writeField(field)
			if err != nil {
				return err
			}
		}
		builder.WriteString("}")
		return nil
	case ast.Enum:
		// All GraphQL enums have underlying type string (in the Go sense).
		builder.WriteString("string\n")
		builder.WriteString("const (\n")
		for _, val := range typedef.EnumValues {
			// TODO: casing should be configurable
			fmt.Fprintf(builder, "%s %s = \"%s\"\n",
				goConstName(val.Name+"_"+builder.typeName),
				builder.typeName, val.Name)
		}
		builder.WriteString(")\n")
		return nil
	case ast.Scalar:
		// TODO(benkraft): Handle custom scalars, unions, and interfaces.
		return fmt.Errorf("not implemented: %v", typedef.Kind)
	default:
		return fmt.Errorf("unexpected kind: %v", typedef.Kind)
	}
}
