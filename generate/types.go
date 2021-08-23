package generate

// This file defines the data structures from which genqlient generates types,
// and the code to write them out as actual Go code.  The main entrypoint is
// goType, which represents such a type, but convert.go also constructs each
// of the implementing types, by traversing the GraphQL operation and schema.

import (
	"fmt"
	"io"
	"strings"
)

// goType represents a type for which we'll generate code.
type goType interface {
	// WriteDefinition writes the code for this type into the given io.Writer.
	//
	// TODO(benkraft): Some of the implementations might now benefit from being
	// converted to templates.
	WriteDefinition(io.Writer, *generator) error

	// Reference returns the Go name of this type, e.g. []*MyStruct, and may be
	// used to refer to it in Go code.
	Reference() string
}

var (
	_ goType = (*goOpaqueType)(nil)
	_ goType = (*goSliceType)(nil)
	_ goType = (*goPointerType)(nil)
	_ goType = (*goEnumType)(nil)
	_ goType = (*goStructType)(nil)
	_ goType = (*goInterfaceType)(nil)
)

type (
	// goOpaqueType represents a user-defined or builtin type, used to
	// represent a GraphQL scalar.
	goOpaqueType struct{ GoRef string }
	// goSliceType represents the Go type []Elem, used to represent GraphQL
	// list types.
	goSliceType struct{ Elem goType }
	// goSliceType represents the Go type *Elem, used when requested by the
	// user (perhaps to handle nulls explicitly, or to avoid copying large
	// structures).
	goPointerType struct{ Elem goType }
)

// Opaque types are defined by the user; pointers and slices need no definition
func (typ *goOpaqueType) WriteDefinition(io.Writer, *generator) error  { return nil }
func (typ *goSliceType) WriteDefinition(io.Writer, *generator) error   { return nil }
func (typ *goPointerType) WriteDefinition(io.Writer, *generator) error { return nil }

func (typ *goOpaqueType) Reference() string  { return typ.GoRef }
func (typ *goSliceType) Reference() string   { return "[]" + typ.Elem.Reference() }
func (typ *goPointerType) Reference() string { return "*" + typ.Elem.Reference() }

// goEnumType represents a Go named-string type used to represent a GraphQL
// enum.  In this case, we generate both the type (`type T string`) and also a
// list of consts representing the values.
type goEnumType struct {
	GoName      string
	Description string
	Values      []goEnumValue
}

type goEnumValue struct {
	Name        string
	Description string
}

func (typ *goEnumType) WriteDefinition(w io.Writer, g *generator) error {
	// All GraphQL enums have underlying type string (in the Go sense).
	writeDescription(w, typ.Description)
	fmt.Fprintf(w, "type %s string\n", typ.GoName)
	fmt.Fprintf(w, "const (\n")
	for _, val := range typ.Values {
		writeDescription(w, val.Description)
		fmt.Fprintf(w, "%s %s = \"%s\"\n",
			typ.GoName+goConstName(val.Name),
			typ.GoName, val.Name)
	}
	fmt.Fprintf(w, ")\n")
	return nil
}

func (typ *goEnumType) Reference() string { return typ.GoName }

// goStructType represents a Go struct type used to represent a GraphQL object
// or input-object type.
type goStructType struct {
	GoName      string
	Description string
	GraphQLName string
	Fields      []*goStructField
	// Incomplete is set if this type contains only certain fields of the
	// corresponding GraphQL type (i.e. those selected by the operation) in
	// which case we put a note in the doc-comment saying as much.
	Incomplete bool
}

type goStructField struct {
	GoName      string
	GoType      goType
	JSONName    string
	Description string
}

func (typ *goStructType) WriteDefinition(w io.Writer, g *generator) error {
	description := typ.Description
	if typ.Incomplete {
		description = incompleteTypeDescription(typ.GoName, typ.GraphQLName, typ.Description)
	}
	writeDescription(w, description)

	fmt.Fprintf(w, "type %s struct {\n", typ.GoName)
	for _, field := range typ.Fields {
		writeDescription(w, field.Description)
		jsonName := field.JSONName
		if _, ok := field.GoType.(*goInterfaceType); ok {
			// abstract types are handled in our UnmarshalJSON
			jsonName = "-"
		}
		fmt.Fprintf(w, "\t%s %s `json:\"%s\"`\n",
			field.GoName, field.GoType.Reference(), jsonName)
	}
	fmt.Fprintf(w, "}\n")

	return g.maybeWriteUnmarshal(w, typ)
}

func (typ *goStructType) Reference() string { return typ.GoName }

// goInterfaceType represents a Go interface type, used to represent a GraphQL
// interface or union type.
type goInterfaceType struct {
	GoName          string
	Description     string
	GraphQLName     string
	Implementations []*goStructType
}

func (typ *goInterfaceType) WriteDefinition(w io.Writer, g *generator) error {
	// TODO(benkraft): also mention the list of implementations.
	description := incompleteTypeDescription(typ.GoName, typ.GraphQLName, typ.Description)
	writeDescription(w, description)

	// Write the interface.
	fmt.Fprintf(w, "type %s interface {\n", typ.GoName)
	implementsMethodName := fmt.Sprintf("implementsGraphQLInterface%v", typ.GoName)
	// TODO(benkraft): Also write GetX() accessor methods for fields of the interface
	fmt.Fprintf(w, "\t%s()\n", implementsMethodName)
	fmt.Fprintf(w, "}\n")

	// Now, write out the implementations.
	for _, impl := range typ.Implementations {
		fmt.Fprintf(w, "func (v %s) %s() {}\n",
			impl.Reference(), implementsMethodName)
	}

	return nil
}

func (typ *goInterfaceType) Reference() string { return typ.GoName }

func incompleteTypeDescription(goName, graphQLName, description string) string {
	// For types where we only have some fields, note that, along with
	// the GraphQL documentation (if any).  We don't want to just use
	// the GraphQL documentation, since it may refer to fields we
	// haven't selected, say.
	prefix := fmt.Sprintf(
		"%v includes the requested fields of the GraphQL type %v.",
		goName, graphQLName)
	if description != "" {
		return fmt.Sprintf(
			"%v\nThe GraphQL type's documentation follows.\n\n%v",
			prefix, description)
	}
	return prefix
}

func writeDescription(w io.Writer, desc string) {
	if desc != "" {
		for _, line := range strings.Split(desc, "\n") {
			fmt.Fprintf(w, "// %s\n", strings.TrimLeft(line, " \t"))
		}
	}
}
