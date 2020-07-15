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

func (g *generator) addFragment(frag *ast.FragmentDefinition) error {
	g.fragments = append(g.fragments, frag)

	selectionSet, err := selections(frag.SelectionSet)
	if err != nil {
		return err
	}

	_, err = g.addTypeForDefinition(upperFirst(frag.Name), frag.Definition, selectionSet)
	return err
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
	err := builder.writeType(typ, g.selectionsForInputType(typ), false)
	return builder.String(), err
}

type selection interface {
	Write(builder *typeBuilder) error
}

func selections(selectionSet ast.SelectionSet) ([]selection, error) {
	retval := make([]selection, len(selectionSet))
	for i, selection := range selectionSet {
		switch selection := selection.(type) {
		case *ast.Field:
			retval[i] = field{selection}
		case *ast.FragmentSpread:
			retval[i] = fragmentSpread{selection}
		case *ast.InlineFragment:
			return nil, fmt.Errorf("not implemented: %T", selection)
		default:
			return nil, fmt.Errorf("invalid selection type: %v", selection)
		}
	}
	return retval, nil
}

type field struct{ field *ast.Field }

func (s field) Write(builder *typeBuilder) error {
	if s.field.Definition == nil || s.field.Definition.Type == nil {
		// Unclear why gqlparser hasn't already rejected this,
		// but empirically it might not.
		return fmt.Errorf("undefined field %v", s.field.Name)
	}

	jsonName := s.field.Alias
	if jsonName == "" {
		// TODO(benkraft): Does this actually happen?  Tests suggest not.
		jsonName = s.field.Name
	}

	selectionSet, err := selections(s.field.SelectionSet)
	if err != nil {
		return err
	}

	return builder.writeField(jsonName, s.field.Definition.Type, selectionSet)
}

type fragmentSpread struct{ frag *ast.FragmentSpread }

func (s fragmentSpread) Write(builder *typeBuilder) error {
	builder.WriteString(upperFirst(s.frag.Name))
	return nil
}

type inputField struct {
	g     *generator
	field *ast.FieldDefinition
}

func (s inputField) Write(builder *typeBuilder) error {
	selectionSet := s.g.selectionsForInputType(s.field.Type)
	return builder.writeField(s.field.Name, s.field.Type, selectionSet)
}

func (g *generator) selectionsForInputType(typ *ast.Type) []selection {
	def := g.schema.Types[typ.Name()]
	selectionSet := make([]selection, len(def.Fields))
	for i, field := range def.Fields {
		selectionSet[i] = inputField{g, field}
	}
	return selectionSet
}

func (builder *typeBuilder) writeField(jsonName string, typ *ast.Type, selectionSet []selection) error {
	// We need an exportable name for JSON-marshaling.
	goName := upperFirst(jsonName)

	builder.WriteString(goName)
	builder.WriteRune(' ')

	err := builder.writeType(typ, selectionSet, true)
	if err != nil {
		return err
	}

	if jsonName != goName {
		fmt.Fprintf(builder, " `json:\"%s\"`", jsonName)
	}
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
			err := field.Write(builder)
			builder.WriteRune('\n')
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
