package generate

import (
	"fmt"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
)

type typeBuilder struct {
	typeName       string
	typeNamePrefix string
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
		return "", fmt.Errorf("%s defined twice:\n%s", name, def)
	}

	fields, err := selections(operation.SelectionSet)
	if err != nil {
		return "", err
	}

	return g.addTypeForDefinition(
		operation.Name, name, g.baseTypeForOperation(operation.Operation), fields)
}

var builtinTypes = map[string]string{
	// GraphQL guarantees int32 is enough, but using int seems more idiomatic
	"Int":     "int",
	"Float":   "float64",
	"String":  "string",
	"Boolean": "bool",
	"ID":      "string",
}

func (g *generator) addTypeForDefinition(namePrefix, nameOverride string, typ *ast.Definition, fields []field) (name string, err error) {
	// If this is a builtin type, just refer to it.
	goName, ok := builtinTypes[typ.Name]
	if ok {
		return goName, nil
	}

	if nameOverride != "" {
		// if we have an explicit name, the passed-in prefix is what we
		// propagate forward
		name = nameOverride
	} else {
		typeGoName := upperFirst(typ.Name)
		if typ.Kind == ast.Enum || typ.Kind == ast.InputObject {
			// If we're an enum or an input-object, there is only one type we
			// will ever possibly generate for this type, so we don't need any
			// of the qualifiers.  This is especially helpful because the
			// caller is very likely to need to reference these types in their
			// code.
			name = typeGoName
		} else if strings.HasSuffix(namePrefix, typeGoName) {
			// If the field and type names are the same, we can avoid the
			// duplication.  (We include the field name in case there are
			// multiple fields with the same type, and the type name because
			// that's the actual name (the rest are really qualifiers); but if
			// they are the same then including it once suffices for both
			// purposes.)
			name = namePrefix
		} else {
			name = namePrefix + typeGoName
		}

		if typ.Kind != ast.Interface && typ.Kind != ast.Union {
			// for interface/union types, we do not add the type name to the
			// name prefix; we want to have QueryFieldType rather than
			// QueryFieldInterfaceType.  Otherwise, the name will also be the
			// prefix for the next type.
			namePrefix = name
		}
		// TODO: for input and enum types, we can probably skip the prefix
		// entirely; they don't have fields that may be used in some places but
		// not others.

	}

	// Otherwise, build the type, put that in the type-map, and return its
	// name.
	builder := &typeBuilder{typeName: name, typeNamePrefix: namePrefix, generator: g}
	fmt.Fprintf(builder, "type %s ", name)
	err = builder.writeTypedef(typ, fields)
	if err != nil {
		return "", err
	}
	// TODO: this should also check for conflicts (except not for enums and
	// input-objects, see above)
	g.typeMap[name] = builder.String()
	return name, nil
}

func (g *generator) getTypeForInputType(opName string, typ *ast.Type) (string, error) {
	// Sort of a hack: case the input type name to match the op-name.
	name := matchFirst(typ.Name(), opName)
	// TODO: we have to pass name 4 times, yuck
	builder := &typeBuilder{typeName: name, typeNamePrefix: name, generator: g}
	err := builder.writeType(name, name, typ, selectionsForType(g, typ))
	return builder.String(), err
}

type field interface {
	Alias() string
	Type() *ast.Type
	SubFields() ([]field, error)
}

type outputField struct{ field *ast.Field }

func (s outputField) Alias() string {
	// gqlparser sets Alias even if the field is not aliased, see e.g.
	// https://github.com/vektah/gqlparser/v2/blob/c06d8e0d135f285e37e7f1ff397f10e049733eb3/parser/query.go#L150
	return s.field.Alias
}

func (s outputField) Type() *ast.Type {
	if s.field.Definition == nil {
		return nil
	}
	return s.field.Definition.Type
}

func (s outputField) SubFields() ([]field, error) {
	return selections(s.field.SelectionSet)
}

func selections(selectionSet ast.SelectionSet) ([]field, error) {
	retval := make([]field, len(selectionSet))
	for i, selection := range selectionSet {
		switch selection := selection.(type) {
		case *ast.Field:
			retval[i] = outputField{selection}
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
func (s inputField) Type() *ast.Type { return s.field.Type }

func (s inputField) SubFields() ([]field, error) {
	return selectionsForType(s.generator, s.field.Type), nil
}

func selectionsForType(g *generator, typ *ast.Type) []field {
	def := g.schema.Types[typ.Name()]
	fields := make([]field, len(def.Fields))
	for i, field := range def.Fields {
		fields[i] = inputField{g, field}
	}
	return fields
}

func (builder *typeBuilder) writeField(field field) error {
	jsonName := field.Alias()
	// We need an exportable name for JSON-marshaling.
	goName := upperFirst(jsonName)

	builder.WriteString(goName)
	builder.WriteRune(' ')

	typ := field.Type()
	if typ == nil {
		// Unclear why gqlparser hasn't already rejected this,
		// but empirically it might not.
		return fmt.Errorf("undefined field %v", field.Alias())
	}

	fields, err := field.SubFields()
	if err != nil {
		return err
	}

	err = builder.writeType(
		// Note we don't deduplicate suffixes here -- if our prefix is GetUser
		// and the field name is User, we do GetUserUser.  This is important
		// because if you have a field called user on a type called User we
		// need `query q { user { user { id } } }` to generate two types, QUser
		// and QUserUser.
		// Note also this is the alias, not the field-name, because if we have
		// `query q { a: f { b }, c: f { d } }` we need separate types for a
		// and c, even though they are the same type in GraphQL, because they
		// have different fields.
		builder.typeNamePrefix+upperFirst(field.Alias()), "", typ, fields)
	if err != nil {
		return err
	}

	if builder.schema.Types[typ.Name()].IsAbstractType() {
		// abstract types are handled in our UnmarshalJSON
		builder.WriteString(" `json:\"-\"`")
	} else if jsonName != goName {
		fmt.Fprintf(builder, " `json:\"%s\"`", jsonName)
	}
	builder.WriteRune('\n')
	return nil
}

func (builder *typeBuilder) writeType(namePrefix, nameOverride string, typ *ast.Type, fields []field) error {
	// gqlgen does slightly different things here, but its implementation may
	// be useful to crib from:
	// https://github.com/99designs/gqlgen/blob/master/plugin/modelgen/models.go#L113
	if typ.Elem != nil {
		// Type is a list.
		builder.WriteString("[]")
		typ = typ.Elem
	}
	// TODO: allow an option to make the Go type a pointer, if you want to do
	// optionality that way, or perhaps others
	// if !typ.NonNull { builder.WriteString("*") }

	def := builder.schema.Types[typ.Name()]
	// Writes a typedef elsewhere (if not already defined)
	name, err := builder.addTypeForDefinition(namePrefix, nameOverride, def, fields)
	if err != nil {
		return err
	}

	builder.WriteString(name)
	return nil
}

func (builder *typeBuilder) writeTypedef(typedef *ast.Definition, fields []field) error {
	switch typedef.Kind {
	case ast.Object, ast.InputObject:
		builder.WriteString("struct {\n")
		for _, field := range fields {
			err := builder.writeField(field)
			if err != nil {
				return err
			}
		}
		builder.WriteString("}")

		// If any field is abstract, we need an UnmarshalJSON method to handle
		// it.
		return builder.maybeWriteUnmarshal(fields)

	case ast.Interface, ast.Union:
		if !allowBrokenFeatures {
			return fmt.Errorf("not implemented: %v", typedef.Kind)
		}

		// First, write the interface type.
		builder.WriteString("interface {\n")
		implementsMethodName := fmt.Sprintf("implementsGraphQLInterface%v", builder.typeName)
		// TODO: Also write GetX() accessor methods for fields of the interface
		builder.WriteString(implementsMethodName)
		builder.WriteString("()\n")
		builder.WriteString("}")

		// Then, write the implementations.
		// TODO(benkraft): Put a doc-comment somewhere with the list.
		for _, impldef := range builder.schema.GetPossibleTypes(typedef) {
			name, err := builder.addTypeForDefinition(builder.typeNamePrefix, "", impldef, fields)
			if err != nil {
				return err
			}

			builder.typeMap[name] += fmt.Sprintf(
				"\nfunc (v %v) %v() {}", name, implementsMethodName)
		}

		return nil

	case ast.Enum:
		// All GraphQL enums have underlying type string (in the Go sense).
		builder.WriteString("string\n")
		builder.WriteString("const (\n")
		for _, val := range typedef.EnumValues {
			fmt.Fprintf(builder, "%s %s = \"%s\"\n",
				builder.typeNamePrefix+goConstName(val.Name),
				builder.typeName, val.Name)
		}
		builder.WriteString(")\n")
		return nil
	case ast.Scalar:
		// TODO(benkraft): Handle custom scalars.
		return fmt.Errorf("not implemented: %v", typedef.Kind)
	default:
		return fmt.Errorf("unexpected kind: %v", typedef.Kind)
	}
}
