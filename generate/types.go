package generate

// This file is the core of genqlient: it's what generates the types into which
// we will unmarshal.
//
// TODO: this file really really needs a file-comment explaining what's going
// on.  probably writing that will help me figure out how to make it make more
// sense!

import (
	"fmt"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
)

type typeBuilder struct {
	strings.Builder
	*generator
}

func (g *generator) baseTypeForOperation(operation ast.Operation) (*ast.Definition, error) {
	switch operation {
	case ast.Query:
		return g.schema.Query, nil
	case ast.Mutation:
		return g.schema.Mutation, nil
	case ast.Subscription:
		if !allowBrokenFeatures {
			return nil, errorf(nil, "genqlient does not yet support subscriptions")
		}
		return g.schema.Subscription, nil
	default:
		return nil, errorf(nil, "unexpected operation: %v", operation)
	}
}

func (g *generator) getTypeForOperation(operation *ast.OperationDefinition, queryOptions *GenqlientDirective) (name string, err error) {
	// TODO: configure ResponseName format
	name = operation.Name + "Response"

	if def, ok := g.typeMap[name]; ok {
		// TODO: check for and handle conflicts a better way
		return "", errorf(operation.Position, "%s defined twice:\n%s", name, def)
	}

	fields, err := selections(g, operation.SelectionSet, queryOptions)
	if err != nil {
		return "", err
	}

	baseType, err := g.baseTypeForOperation(operation.Operation)
	if err != nil {
		return "", errorf(operation.Position, "%v", err)
	}

	description := fmt.Sprintf("%v is returned by %v on success.", name, operation.Name)

	builder := &typeBuilder{generator: g}
	err = builder.writeTypedef(
		name, operation.Name, baseType, operation.Position, fields, queryOptions, description)
	return name, err
}

var builtinTypes = map[string]string{
	// GraphQL guarantees int32 is enough, but using int seems more idiomatic
	"Int":     "int",
	"Float":   "float64",
	"String":  "string",
	"Boolean": "bool",
	"ID":      "string",
}

func (g *generator) typeName(prefix string, typ *ast.Definition) (name, nextPrefix string) {
	typeGoName := upperFirst(typ.Name)
	if typ.Kind == ast.Enum || typ.Kind == ast.InputObject {
		// If we're an enum or an input-object, there is only one type we
		// will ever possibly generate for this type, so we don't need any
		// of the qualifiers.  This is especially helpful because the
		// caller is very likely to need to reference these types in their
		// code.
		return typeGoName, typeGoName
	}

	name = prefix
	if !strings.HasSuffix(prefix, typeGoName) {
		// If the field and type names are the same, we can avoid the
		// duplication.  (We include the field name in case there are
		// multiple fields with the same type, and the type name because
		// that's the actual name (the rest are really qualifiers); but if
		// they are the same then including it once suffices for both
		// purposes.)
		// TODO: do this a bit more fuzzily; for example if you have a field
		//  doThing: DoThingMutation
		// or
		//	error: MyError
		// we should be able to be a bit smarter than
		// DoThingDoThingMutation/ErrorMyError.
		name += typeGoName
	}

	if typ.Kind == ast.Interface || typ.Kind == ast.Union {
		// for interface/union types, we do not add the type name to the
		// name prefix; we want to have QueryFieldType rather than
		// QueryFieldInterfaceType.  So we just use the input prefix.
		return name, prefix
	}

	// Otherwise, the name will also be the prefix for the next type.
	return name, name
}

func (g *generator) getTypeForInputType(opName string, typ *ast.Type, options, queryOptions *GenqlientDirective) (string, error) {
	// Sort of a hack: case the input type name to match the op-name.
	name := matchFirst(typ.Name(), opName)
	builder := &typeBuilder{generator: g}
	// note prefix is ignored here (see generator.typeName)
	// TODO: passing options is actually kinda wrong, because it means we could
	// break the "there is only Go type for each input type" rule.  In practice
	// it's probably rare that you use the same input type twice in a query and
	// want different settings, though, and it just means we choose one or the
	// other set of options.
	// TODO: it's also awkward because you have no way to pass an option for an
	// individual input-type field.
	// TODO: should we use pointers by default for input-types if they're
	// structs?
	err := builder.writeType(name, "", typ, selectionsForInputType(g, typ, queryOptions), options)
	return builder.String(), err
}

type field interface {
	Alias() string
	Options() (*GenqlientDirective, error)
	Description() string
	Type() *ast.Type
	Pos() *ast.Position
	SubFields() ([]field, error)
}

type outputField struct {
	*generator
	queryOptions *GenqlientDirective
	field        *ast.Field
}

func (s outputField) Alias() string {
	// gqlparser sets Alias even if the field is not aliased, see e.g.
	// https://github.com/vektah/gqlparser/v2/blob/c06d8e0d135f285e37e7f1ff397f10e049733eb3/parser/query.go#L150
	return s.field.Alias
}

func (s outputField) Options() (*GenqlientDirective, error) {
	_, directive, err := s.generator.parsePrecedingComment(s.field, s.field.Position)
	if err != nil {
		return nil, err
	}
	return s.queryOptions.merge(directive), nil
}

func (s outputField) Description() string {
	if s.field.Definition == nil {
		return ""
	}
	return s.field.Definition.Description
}

func (s outputField) Type() *ast.Type {
	if s.field.Definition == nil {
		return nil
	}
	return s.field.Definition.Type
}

func (s outputField) Pos() *ast.Position {
	return s.field.Position
}

func (s outputField) SubFields() ([]field, error) {
	return selections(s.generator, s.field.SelectionSet, s.queryOptions)
}

func selections(g *generator, selectionSet ast.SelectionSet, options *GenqlientDirective) ([]field, error) {
	retval := make([]field, len(selectionSet))
	for i, selection := range selectionSet {
		switch selection := selection.(type) {
		case *ast.Field:
			retval[i] = outputField{g, options, selection}
		case *ast.FragmentSpread:
			return nil, errorf(selection.Position, "not implemented: %T", selection)
		case *ast.InlineFragment:
			return nil, errorf(selection.Position, "not implemented: %T", selection)
		default:
			return nil, errorf(nil, "invalid selection type: %T", selection)
		}
	}
	return retval, nil
}

type inputField struct {
	*generator
	field        *ast.FieldDefinition
	queryOptions *GenqlientDirective
}

func (s inputField) Alias() string { return s.field.Name }
func (s inputField) Options() (*GenqlientDirective, error) {
	_, directive, err := s.generator.parsePrecedingComment(s.field, s.field.Position)
	if err != nil {
		return nil, err
	}
	return s.queryOptions.merge(directive), nil
}
func (s inputField) Description() string { return s.field.Description }
func (s inputField) Type() *ast.Type     { return s.field.Type }
func (s inputField) Pos() *ast.Position  { return s.field.Position }

func (s inputField) SubFields() ([]field, error) {
	return selectionsForInputType(s.generator, s.field.Type, s.queryOptions), nil
}

func selectionsForInputType(g *generator, typ *ast.Type, queryOptions *GenqlientDirective) []field {
	def := g.schema.Types[typ.Name()]
	fields := make([]field, len(def.Fields))
	for i, field := range def.Fields {
		fields[i] = inputField{g, field, queryOptions}
	}
	return fields
}

func (builder *typeBuilder) writeField(typeNamePrefix string, field field) error {
	jsonName := field.Alias()
	// We need an exportable name for JSON-marshaling.
	goName := upperFirst(jsonName)

	typ := field.Type()
	if typ == nil {
		// Unclear why gqlparser hasn't already rejected this,
		// but empirically it might not.
		return errorf(field.Pos(), "undefined field %v", field.Alias())
	}

	fields, err := field.SubFields()
	if err != nil {
		return err
	}

	options, err := field.Options()
	if err != nil {
		return err
	}

	typedef := builder.schema.Types[typ.Name()]

	builder.writeDescription(field.Description())
	builder.WriteString(goName)
	builder.WriteRune(' ')

	// Note we don't deduplicate suffixes here -- if our prefix is GetUser
	// and the field name is User, we do GetUserUser.  This is important
	// because if you have a field called user on a type called User we
	// need `query q { user { user { id } } }` to generate two types, QUser
	// and QUserUser.
	// Note also this is named based on the GraphQL alias (Go name), not the
	// field-name, because if we have `query q { a: f { b }, c: f { d } }` we
	// need separate types for a and c, even though they are the same type in
	// GraphQL, because they have different fields.
	name, namePrefix := builder.typeName(typeNamePrefix+goName, typedef)
	err = builder.writeType(name, namePrefix, typ, fields, options)
	if err != nil {
		return err
	}

	if typedef.IsAbstractType() {
		// abstract types are handled in our UnmarshalJSON
		jsonName = "-"
	}

	fmt.Fprintf(builder, " `json:\"%s\"`\n", jsonName)
	return nil
}

func (builder *typeBuilder) writeType(name, namePrefix string, typ *ast.Type, fields []field, options *GenqlientDirective) error {
	// gqlgen does slightly different things here, but its implementation may
	// be useful to crib from:
	// https://github.com/99designs/gqlgen/blob/master/plugin/modelgen/models.go#L113
	for typ.Elem != nil {
		// Type is a list.
		builder.WriteString("[]")
		typ = typ.Elem
	}
	if options.GetPointer() {
		// TODO: this does []*T, you might in principle want *[]T or
		// *[]*T.  We could add a "sliceptr" option if it comes up (that's
		// still not correct if you wanted *[][]*[]T, but, like, tough luck).
		builder.WriteString("*")
	}

	// If this is a builtin type or custom scalar, just refer to it.
	def := builder.schema.Types[typ.Name()]
	goName, ok := builder.Config.Scalars[def.Name]
	if ok {
		name, err := builder.addRef(goName)
		builder.WriteString(name)
		return err
	}
	goName, ok = builtinTypes[def.Name]
	if ok {
		builder.WriteString(goName)
		return nil
	}

	// Else, write the name, then generate the definition.
	builder.WriteString(name)

	childBuilder := &typeBuilder{generator: builder.generator}
	return childBuilder.writeTypedef(name, namePrefix, def, typ.Position, fields, options, "")
}

func (builder *typeBuilder) writeTypedef(
	typeName, typeNamePrefix string,
	typedef *ast.Definition,
	pos *ast.Position,
	fields []field,
	options *GenqlientDirective,
	description string, // defaults to typedef.Description
) (err error) {
	defer func() {
		// Whenever we're done, add the type to the type-map.
		// TODO: there's got to be a better way than defer.
		if err == nil {
			// TODO: this should also check for conflicts (except not for enums
			// and input-objects, see above)
			builder.typeMap[typeName] = builder.String()
		}
	}()

	if description == "" {
		description = typedef.Description
	}
	builder.writeDescription(description)

	fmt.Fprintf(builder, "type %s ", typeName)
	switch typedef.Kind {
	case ast.Object, ast.InputObject:
		builder.WriteString("struct {\n")
		for _, field := range fields {
			err := builder.writeField(typeNamePrefix, field)
			if err != nil {
				return err
			}
		}
		builder.WriteString("}")

		// If any field is abstract, we need an UnmarshalJSON method to handle
		// it.
		return builder.maybeWriteUnmarshal(typeName, typeNamePrefix, fields)

	case ast.Interface, ast.Union:
		if !allowBrokenFeatures {
			return errorf(pos, "not implemented: %v", typedef.Kind)
		}

		// First, write the interface type.
		builder.WriteString("interface {\n")
		implementsMethodName := fmt.Sprintf("implementsGraphQLInterface%v", typeName)
		// TODO: Also write GetX() accessor methods for fields of the interface
		builder.WriteString(implementsMethodName)
		builder.WriteString("()\n")
		builder.WriteString("}")

		// Then, write the implementations.
		// TODO(benkraft): Put a doc-comment somewhere with the list.
		for _, impldef := range builder.schema.GetPossibleTypes(typedef) {
			name, namePrefix := builder.typeName(typeNamePrefix, impldef)
			implBuilder := &typeBuilder{generator: builder.generator}
			err := implBuilder.writeTypedef(name, namePrefix, impldef, pos, fields, options, "")
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
			builder.writeDescription(val.Description)
			fmt.Fprintf(builder, "%s %s = \"%s\"\n",
				typeName+goConstName(val.Name),
				typeName, val.Name)
		}
		builder.WriteString(")\n")
		return nil
	case ast.Scalar:
		return errorf(pos, "unknown scalar %v: please add it to genqlient.yaml", typedef.Name)
	default:
		return errorf(pos, "unexpected kind: %v", typedef.Kind)
	}
}

func (builder *typeBuilder) writeDescription(desc string) {
	if desc != "" {
		for _, line := range strings.Split(desc, "\n") {
			builder.WriteString("// " + strings.TrimLeft(line, " \t") + "\n")
		}
	}
}
