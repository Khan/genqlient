package generate

// This file implements the core type-generation logic of genqlient, whereby we
// traverse an operation-definition (and the schema against which it will be
// executed), and convert that into Go types.  It returns data structures
// representing the types to be generated; these are defined, and converted
// into code, in types.go.
//
// The entrypoints are convertOperation, which builds the response-type for a
// query, and convertInputType, which builds the argument-types.

import (
	"fmt"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
)

// baseTypeForOperation returns the definition of the GraphQL type to which the
// root of the operation corresponds, e.g. the "Query" or "Mutation" type.
func (g *generator) baseTypeForOperation(operation ast.Operation) (*ast.Definition, error) {
	switch operation {
	case ast.Query:
		return g.schema.Query, nil
	case ast.Mutation:
		return g.schema.Mutation, nil
	case ast.Subscription:
		if !g.Config.AllowBrokenFeatures {
			return nil, errorf(nil, "genqlient does not yet support subscriptions")
		}
		return g.schema.Subscription, nil
	default:
		return nil, errorf(nil, "unexpected operation: %v", operation)
	}
}

// convertOperation builds the response-type into which the given operation's
// result will be unmarshaled.
func (g *generator) convertOperation(
	operation *ast.OperationDefinition,
	queryOptions *GenqlientDirective,
) (goType, error) {
	name := operation.Name + "Response"

	if def, ok := g.typeMap[name]; ok {
		return nil, errorf(operation.Position, "%s defined twice:\n%s", name, def)
	}

	baseType, err := g.baseTypeForOperation(operation.Operation)
	if err != nil {
		return nil, errorf(operation.Position, "%v", err)
	}

	goTyp, err := g.convertDefinition(
		name, operation.Name, baseType, operation.Position,
		operation.SelectionSet, queryOptions, queryOptions)

	if structType, ok := goTyp.(*goStructType); ok {
		// Override the ordinary description; the GraphQL documentation for
		// Query/Mutation is unlikely to be of value.
		// TODO(benkraft): This is a bit awkward/fragile.
		structType.Description =
			fmt.Sprintf("%v is returned by %v on success.", name, operation.Name)
		structType.Incomplete = false
	}

	return goTyp, err
}

var builtinTypes = map[string]string{
	// GraphQL guarantees int32 is enough, but using int seems more idiomatic
	"Int":     "int",
	"Float":   "float64",
	"String":  "string",
	"Boolean": "bool",
	"ID":      "string",
}

// typeName computes the name, in Go, that we should use for the given
// GraphQL type definition.  This is dependent on its location within the query
// (see DESIGN.md for more on why we generate type-names this way), which is
// determined by the prefix argument; the nextPrefix result should be passed to
// calls to typeName on any child types.
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

// convertInputType decides the Go type we will generate corresponding to an
// argument to a GraphQL operation.
func (g *generator) convertInputType(
	opName string,
	typ *ast.Type,
	options, queryOptions *GenqlientDirective,
) (goType, error) {
	// Sort of a hack: case the input type name to match the op-name.
	// TODO(benkraft): this is another thing that breaks the assumption that we
	// only need one of an input type, albeit in a relatively safe way.
	name := matchFirst(typ.Name(), opName)
	// note prefix is ignored here (see generator.typeName), as is selectionSet
	// (for input types we use the whole thing)).
	return g.convertType(name, "", typ, nil, options, queryOptions)
}

// convertType decides the Go type we will generate corresponding to a
// particular GraphQL type.  In this context, "type" represents the type of a
// field, and may be a list or a reference to a named type, with or without the
// "non-null" annotation.
func (g *generator) convertType(
	name, namePrefix string,
	typ *ast.Type,
	selectionSet ast.SelectionSet,
	options, queryOptions *GenqlientDirective,
) (goType, error) {
	// We check for local bindings here, so that you can bind, say, a
	// `[String!]` to a struct instead of a slice.  Global bindings can only
	// bind GraphQL named types, at least for now.
	localBinding := options.Bind
	if localBinding != "" && localBinding != "-" {
		goRef, err := g.addRef(localBinding)
		return &goOpaqueType{goRef}, err
	}

	if typ.Elem != nil {
		// Type is a list.
		elem, err := g.convertType(
			name, namePrefix, typ.Elem, selectionSet, options, queryOptions)
		return &goSliceType{elem}, err
	}

	// If this is a builtin type or custom scalar, just refer to it.
	def := g.schema.Types[typ.Name()]
	goTyp, err := g.convertDefinition(
		name, namePrefix, def, typ.Position, selectionSet, options, queryOptions)

	if options.GetPointer() {
		// Whatever we get, wrap it in a pointer.  (Because of the way the
		// options work, recursing here isn't as connvenient.)
		// Note this does []*T or [][]*T, not e.g. *[][]T.  See #16.
		goTyp = &goPointerType{goTyp}
	}
	return goTyp, err
}

// convertDefinition decides the Go type we will generate corresponding to a
// particular GraphQL named type.
//
// In this context, "definition" (and "named type") refer to an
// *ast.Definition, which represents the definition of a type in the GraphQL
// schema, which may be referenced by a field-type (see convertType).
func (g *generator) convertDefinition(
	name, namePrefix string,
	def *ast.Definition,
	pos *ast.Position,
	selectionSet ast.SelectionSet,
	options, queryOptions *GenqlientDirective,
) (goType, error) {
	// Check if we should use an existing type.  (This is usually true for
	// GraphQL scalars, but we allow you to bind non-scalar types too, if you
	// want, subject to the caveats described in Config.Bindings.)  Local
	// bindings are checked in the caller (convertType) and never get here,
	// unless the binding is "-" which means "ignore the global binding".
	globalBinding, ok := g.Config.Bindings[def.Name]
	if ok && options.Bind != "-" {
		goRef, err := g.addRef(globalBinding.Type)
		return &goOpaqueType{goRef}, err
	}
	goBuiltinName, ok := builtinTypes[def.Name]
	if ok {
		return &goOpaqueType{goBuiltinName}, nil
	}

	switch def.Kind {
	case ast.Object:
		fields, err := g.convertSelectionSet(
			namePrefix, selectionSet, def, queryOptions)
		if err != nil {
			return nil, err
		}

		goType := &goStructType{
			GoName:      name,
			Description: def.Description,
			GraphQLName: def.Name,
			Fields:      fields,
			Incomplete:  true,
		}
		g.typeMap[name] = goType
		return goType, nil

	case ast.InputObject:
		goType := &goStructType{
			GoName:      name,
			Description: def.Description,
			GraphQLName: def.Name,
			Fields:      make([]*goStructField, len(def.Fields)),
		}
		g.typeMap[name] = goType

		for i, field := range def.Fields {
			goName := upperFirst(field.Name)
			// Several of the arguments don't really make sense here:
			// - no field-specific options can apply, because this is
			//   a field in the type, not in the query (see also #14).
			// - namePrefix is ignored for input types; see note in
			//   generator.typeName.
			// TODO(benkraft): Can we refactor to avoid passing the values that
			// will be ignored?  We know field.Type is a scalar, enum, or input
			// type.  But plumbing that is a bit tricky in practice.
			fieldGoType, err := g.convertType(
				field.Type.Name(), "", field.Type, nil, queryOptions, queryOptions)
			if err != nil {
				return nil, err
			}

			goType.Fields[i] = &goStructField{
				GoName:      goName,
				GoType:      fieldGoType,
				JSONName:    field.Name,
				GraphQLName: field.Name,
				Description: field.Description,
			}
		}
		return goType, nil

	case ast.Interface, ast.Union:
		sharedFields, err := g.convertSelectionSet(
			namePrefix, selectionSet, def, queryOptions)
		if err != nil {
			return nil, err
		}

		implementationTypes := g.schema.GetPossibleTypes(def)
		goType := &goInterfaceType{
			GoName:          name,
			Description:     def.Description,
			GraphQLName:     def.Name,
			SharedFields:    sharedFields,
			Implementations: make([]*goStructType, len(implementationTypes)),
		}
		g.typeMap[name] = goType

		for i, implDef := range implementationTypes {
			// Note for shared fields we propagate forward the interface's
			// name-prefix: that is, the implementations will have fields with
			// types like
			//	MyInterfaceMyFieldMyType
			// not
			//	MyInterfaceMyImplMyFieldMyType
			//             ^^^^^^
			// In particular, this means that the Go type of MyField will be
			// the same across all the implementations; this is important so
			// that we can write a method GetMyField() that returns it!
			implName, _ := g.typeName(namePrefix, implDef)
			// TODO(benkraft): In principle we should skip generating a Go
			// field for __typename each of these impl-defs if you didn't
			// request it (and it was automatically added by
			// preprocessQueryDocument).  But in practice it doesn't really
			// hurt, and would be extra work to avoid, so we just leave it.
			implTyp, err := g.convertDefinition(
				implName, namePrefix, implDef, pos, selectionSet, options, queryOptions)
			if err != nil {
				return nil, err
			}

			implStructTyp, ok := implTyp.(*goStructType)
			if !ok { // (should never happen on a valid schema)
				return nil, errorf(
					pos, "interface %s had non-object implementation %s",
					def.Name, implDef.Name)
			}
			goType.Implementations[i] = implStructTyp
		}
		return goType, nil

	case ast.Enum:
		goType := &goEnumType{
			GoName:      name,
			Description: def.Description,
			Values:      make([]goEnumValue, len(def.EnumValues)),
		}
		g.typeMap[name] = goType

		for i, val := range def.EnumValues {
			goType.Values[i] = goEnumValue{Name: val.Name, Description: val.Description}
		}
		return goType, nil

	case ast.Scalar:
		// (If you had an entry in bindings, we would have returned it above.)
		return nil, errorf(
			pos, `unknown scalar %v: please add it to "bindings" in genqlient.yaml`, def.Name)
	default:
		return nil, errorf(pos, "unexpected kind: %v", def.Kind)
	}
}

// convertSelectionSet converts a GraphQL selection-set into a list of
// corresponding Go struct-fields (and their Go types)
//
// A selection-set is a list of fields within braces like `{ myField }`, as
// appears at the toplevel of a query, in a field's sub-selections, or within
// an inline or named fragment.
//
// containingTypedef is the type-def whose fields we are selecting, and may be
// an object type or an interface type.  In the case of interfaces, we'll call
// convertSelectionSet once for the interface, and once for each
// implementation.
func (g *generator) convertSelectionSet(
	namePrefix string,
	selectionSet ast.SelectionSet,
	containingTypedef *ast.Definition,
	queryOptions *GenqlientDirective,
) ([]*goStructField, error) {
	fields := make([]*goStructField, 0, len(selectionSet))
	for _, selection := range selectionSet {
		_, selectionDirective, err := g.parsePrecedingComment(
			selection, selection.GetPosition())
		if err != nil {
			return nil, err
		}
		selectionOptions := queryOptions.merge(selectionDirective)

		switch selection := selection.(type) {
		case *ast.Field:
			field, err := g.convertField(
				namePrefix, selection, selectionOptions, queryOptions)
			if err != nil {
				return nil, err
			}
			fields = append(fields, field)
		case *ast.FragmentSpread:
			return nil, errorf(selection.Position, "not implemented: %T", selection)
		case *ast.InlineFragment:
			// (Note this will return nil, nil if the fragment doesn't apply to
			// this type.)
			fragmentFields, err := g.convertInlineFragment(
				namePrefix, selection, containingTypedef, queryOptions)
			if err != nil {
				return nil, err
			}
			fields = append(fields, fragmentFields...)
		default:
			return nil, errorf(nil, "invalid selection type: %T", selection)
		}
	}

	// We need to deduplicate, if you asked for
	//	{ id, id, id, ... on SubType { id } }
	// (which, yes, is legal) we'll treat that as just { id }.
	uniqFields := make([]*goStructField, 0, len(selectionSet))
	fieldNames := make(map[string]bool, len(selectionSet))
	for _, field := range fields {
		// GraphQL (and, effectively, JSON) requires that all fields with the
		// same alias (JSON-name) must be the same (i.e. refer to the same
		// field), so that's how we deduplicate.
		if fieldNames[field.JSONName] {
			// GraphQL (and, effectively, JSON) forbids you from having two
			// fields with the same alias (JSON-name) that refer to different
			// GraphQL fields.  But it does allow you to have the same field
			// with different selections (subject to some additional rules).
			// We say: that's too complicated! and allow duplicate fields
			// only if they're "leaf" types (enum or scalar).
			switch field.GoType.Unwrap().(type) {
			case *goOpaqueType, *goEnumType:
				// Leaf field; we can just deduplicate.
				// Note GraphQL already guarantees that the conflicting field
				// has scalar/enum type iff this field does:
				// https://spec.graphql.org/draft/#SameResponseShape()
				continue
			case *goStructType, *goInterfaceType:
				// TODO(benkraft): Keep track of the position of each
				// selection, so we can put this error on the right line.
				return nil, errorf(nil,
					"genqlient doesn't allow duplicate fields with different selections "+
						"(see https://github.com/Khan/genqlient/issues/64); "+
						"duplicate field: %s.%s", containingTypedef.Name, field.JSONName)
			default:
				return nil, errorf(nil, "unexpected field-type: %T", field.GoType.Unwrap())
			}
		}
		uniqFields = append(uniqFields, field)
		fieldNames[field.JSONName] = true
	}
	return uniqFields, nil
}

// fragmentMatches returns true if the given fragment is "active" when applied
// to the given type.
//
// "Active" here means "the fragment's fields will be returned on all objects
// of the given type", which is true when the given type is or implements
// the fragment's type.  This is distinct from the rules for when a fragment
// spread is legal, which is true when the fragment would be active for *any*
// of the concrete types the spread-context could have (see
// https://spec.graphql.org/draft/#sec-Fragment-Spreads or DESIGN.md).
//
// containingTypedef is as described in convertInlineFragment, below.
// fragmentTypedef is the definition of the fragment's type-condition, i.e. the
// definition of MyType in a fragment `on MyType`.
func fragmentMatches(containingTypedef, fragmentTypedef *ast.Definition) bool {
	if containingTypedef.Name == fragmentTypedef.Name {
		return true
	}
	for _, iface := range containingTypedef.Interfaces {
		// Note we don't need to recurse into the interfaces here, because in
		// GraphQL types must list all the interfaces they implement, including
		// all types those interfaces implement [1].  Actually, at present
		// gqlparser doesn't even support interfaces implementing other
		// interfaces, but our code would handle that too.
		// [1] https://spec.graphql.org/draft/#sec-Interfaces.Interfaces-Implementing-Interfaces
		if iface == fragmentTypedef.Name {
			return true
		}
	}
	return false
}

// convertInlineFragment converts a single GraphQL inline fragment
// (`... on MyType { myField }`) into Go struct-fields.
//
// containingTypedef is the type-def corresponding to the type into which we
// are spreading; it may be either an interface type (when spreading into one)
// or an object type (when writing the implementations of such an interface, or
// when using an inline fragment in an object type which is rare).  If the
// given fragment does not apply to that type, this function returns nil, nil.
//
// In general, we treat such fragments' fields as if they were fields of the
// parent selection-set (except of course they are only included in types the
// fragment matches); see DESIGN.md for more.
func (g *generator) convertInlineFragment(
	namePrefix string,
	fragment *ast.InlineFragment,
	containingTypedef *ast.Definition,
	queryOptions *GenqlientDirective,
) ([]*goStructField, error) {
	// You might think fragmentTypedef would be fragment.ObjectDefinition, but
	// actually that's the type into which the fragment is spread.
	fragmentTypedef := g.schema.Types[fragment.TypeCondition]
	if !fragmentMatches(containingTypedef, fragmentTypedef) {
		return nil, nil
	}
	return g.convertSelectionSet(namePrefix, fragment.SelectionSet,
		containingTypedef, queryOptions)
}

// convertField converts a single GraphQL operation-field into a Go
// struct-field (and its type).
//
// Note that input-type fields are handled separately (inline in
// convertDefinition), because they come from the type-definition, not the
// operation.
func (g *generator) convertField(
	namePrefix string,
	field *ast.Field,
	fieldOptions, queryOptions *GenqlientDirective,
) (*goStructField, error) {
	if field.Definition == nil {
		// Unclear why gqlparser hasn't already rejected this,
		// but empirically it might not.
		return nil, errorf(
			field.Position, "undefined field %v", field.Alias)
	}

	// Needs to be exported for JSON-marshaling
	goName := upperFirst(field.Alias)

	typ := field.Definition.Type
	fieldTypedef := g.schema.Types[typ.Name()]

	// Note we don't deduplicate suffixes here -- if our prefix is GetUser and
	// the field name is User, we do GetUserUser.  This is important because if
	// you have a field called user on a type called User we need
	// `query q { user { user { id } } }` to generate two types, QUser and
	// QUserUser.  Note also this is named based on the GraphQL alias (Go
	// name), not the field-name, because if we have
	// `query q { a: f { b }, c: f { d } }` we need separate types for a and c,
	// even though they are the same type in GraphQL, because they have
	// different fields.
	name, namePrefix := g.typeName(namePrefix+goName, fieldTypedef)
	fieldGoType, err := g.convertType(
		name, namePrefix, typ, field.SelectionSet,
		fieldOptions, queryOptions)
	if err != nil {
		return nil, err
	}

	return &goStructField{
		GoName:      goName,
		GoType:      fieldGoType,
		JSONName:    field.Alias,
		GraphQLName: field.Name,
		Description: field.Definition.Description,
	}, nil
}
