package generate

// This file defines the data structures from which genqlient generates types,
// and the code to write them out as actual Go code.  The main entrypoint is
// goType, which represents such a type, but convert.go also constructs each
// of the implementing types, by traversing the GraphQL operation and schema.

import (
	"fmt"
	"io"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
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

	// GraphQLTypeName returns the name of the GraphQL type to which this Go type
	// corresponds.
	GraphQLTypeName() string

	// SelectionSet returns the selection-set of the GraphQL field from which
	// this type was generated, or nil if none is applicable (for GraphQL
	// scalar, enum, and input types, as well as any opaque
	// (non-genqlient-generated) type since those are validated upon creation).
	SelectionSet() ast.SelectionSet

	// Remove slice/pointer wrappers, and return the underlying (named (or
	// builtin)) type.  For example, given []*MyStruct, return MyStruct.
	Unwrap() goType

	// Count the number of times Unwrap() will unwrap a slice type.  For
	// example, given [][][]*MyStruct (or []**[][]*MyStruct, but we never
	// currently generate that), return 3.
	SliceDepth() int

	// True if Unwrap() will unwrap a pointer at least once.
	IsPointer() bool
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
	// goOpaqueType represents a user-defined or builtin type, often used to
	// represent a GraphQL scalar.  (See Config.Bindings for more context.)
	goOpaqueType struct {
		GoRef       string
		GraphQLName string
	}
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

func (typ *goOpaqueType) SelectionSet() ast.SelectionSet  { return nil }
func (typ *goSliceType) SelectionSet() ast.SelectionSet   { return typ.Elem.SelectionSet() }
func (typ *goPointerType) SelectionSet() ast.SelectionSet { return typ.Elem.SelectionSet() }

func (typ *goOpaqueType) GraphQLTypeName() string  { return typ.GraphQLName }
func (typ *goSliceType) GraphQLTypeName() string   { return typ.Elem.GraphQLTypeName() }
func (typ *goPointerType) GraphQLTypeName() string { return typ.Elem.GraphQLTypeName() }

// goEnumType represents a Go named-string type used to represent a GraphQL
// enum.  In this case, we generate both the type (`type T string`) and also a
// list of consts representing the values.
type goEnumType struct {
	GoName      string
	GraphQLName string
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

func (typ *goEnumType) Reference() string              { return typ.GoName }
func (typ *goEnumType) SelectionSet() ast.SelectionSet { return nil }
func (typ *goEnumType) GraphQLTypeName() string        { return typ.GraphQLName }

// goStructType represents a Go struct type used to represent a GraphQL object
// or input-object type.
type goStructType struct {
	GoName    string
	Fields    []*goStructField
	IsInput   bool
	Selection ast.SelectionSet
	descriptionInfo
}

type goStructField struct {
	GoName      string
	GoType      goType
	JSONName    string // i.e. the field's alias in this query
	GraphQLName string // i.e. the field's name in its type-def
	Omitempty   bool   // only used on input types
	Description string
}

// IsAbstract returns true if this field is of abstract type (i.e. GraphQL
// union or interface; equivalently, represented by an interface in Go).
func (field *goStructField) IsAbstract() bool {
	_, ok := field.GoType.Unwrap().(*goInterfaceType)
	return ok
}

// IsEmbedded returns true if this field is embedded (a.k.a. anonymous), which
// is in practice true if it corresponds to a named fragment spread in GraphQL.
func (field *goStructField) IsEmbedded() bool {
	return field.GoName == ""
}

func (typ *goStructType) WriteDefinition(w io.Writer, g *generator) error {
	writeDescription(w, structDescription(typ))

	needUnmarshaler := false
	fmt.Fprintf(w, "type %s struct {\n", typ.GoName)
	for _, field := range typ.Fields {
		writeDescription(w, field.Description)
		jsonTag := `"` + field.JSONName
		if field.Omitempty {
			jsonTag += ",omitempty"
		}
		jsonTag += `"`
		if field.IsAbstract() {
			// abstract types are handled in our UnmarshalJSON (see below)
			needUnmarshaler = true
			jsonTag = `"-"`
		}
		if field.IsEmbedded() {
			// embedded fields also need UnmarshalJSON handling (see below)
			needUnmarshaler = true
			fmt.Fprintf(w, "\t%s `json:\"-\"`\n", field.GoType.Unwrap().Reference())
		} else {
			fmt.Fprintf(w, "\t%s %s `json:%s`\n",
				field.GoName, field.GoType.Reference(), jsonTag)
		}
	}
	fmt.Fprintf(w, "}\n")

	// Now, if needed, write the unmarshaler.  We need one if we have any
	// interface-typed fields, or any embedded fields.
	//
	// For interface-typed fields, ideally we'd write an UnmarshalJSON method
	// on the field, but you can't add a method to an interface.  So we write a
	// per-interface-type helper, but we have to call it (with a little
	// boilerplate) everywhere the type is referenced.
	//
	// For embedded fields (from fragments), mostly the JSON library would just
	// do what we want, but there are two problems.  First, if the embedded
	// type has its own UnmarshalJSON, naively that would be promoted to
	// become our UnmarshalJSON, which is no good.  But we don't want to just
	// hide that method and inline its fields, either; we need to call its
	// UnmarshalJSON (on the same object we unmarshal into this struct).
	// Second, if the embedded type duplicates any fields of the embedding type
	// -- maybe both the fragment and the selection into which it's spread
	// select the same field, or several fragments select the same field -- the
	// JSON library will only fill one of those (the least-nested one); we want
	// to fill them all.
	//
	// TODO(benkraft): If/when proposal #5901 is implemented (Go 1.18 at the
	// earliest), we may be able to do some of this a simpler way.
	if !needUnmarshaler {
		return nil
	}

	return g.render("unmarshal.go.tmpl", w, typ)
}

func (typ *goStructType) Reference() string              { return typ.GoName }
func (typ *goStructType) SelectionSet() ast.SelectionSet { return typ.Selection }
func (typ *goStructType) GraphQLTypeName() string        { return typ.GraphQLName }

// goInterfaceType represents a Go interface type, used to represent a GraphQL
// interface or union type.
type goInterfaceType struct {
	GoName string
	// Fields shared by all the interface's implementations;
	// we'll generate getter methods for each.
	SharedFields    []*goStructField
	Implementations []*goStructType
	Selection       ast.SelectionSet
	descriptionInfo
}

func (typ *goInterfaceType) WriteDefinition(w io.Writer, g *generator) error {
	writeDescription(w, interfaceDescription(typ))

	// Write the interface.
	fmt.Fprintf(w, "type %s interface {\n", typ.GoName)
	implementsMethodName := fmt.Sprintf("implementsGraphQLInterface%v", typ.GoName)
	fmt.Fprintf(w, "\t%s()\n", implementsMethodName)
	for _, sharedField := range typ.SharedFields {
		if sharedField.GoName == "" { // embedded type
			fmt.Fprintf(w, "\t%s\n", sharedField.GoType.Reference())
			continue
		}

		methodName := "Get" + sharedField.GoName
		description := ""
		if sharedField.GraphQLName == "__typename" {
			description = fmt.Sprintf(
				"%s returns the receiver's concrete GraphQL type-name "+
					"(see interface doc for possible values).", methodName)
		} else {
			description = fmt.Sprintf(
				`%s returns the interface-field "%s" from its implementation.`,
				methodName, sharedField.GraphQLName)
			if sharedField.Description != "" {
				description = fmt.Sprintf(
					"%s\nThe GraphQL interface field's documentation follows.\n\n%s",
					description, sharedField.Description)
			}
		}

		writeDescription(w, description)
		fmt.Fprintf(w, "\t%s() %s\n", methodName, sharedField.GoType.Reference())
	}
	fmt.Fprintf(w, "}\n")

	// Now, write out the implementations.
	for _, impl := range typ.Implementations {
		fmt.Fprintf(w, "func (v *%s) %s() {}\n",
			impl.Reference(), implementsMethodName)
		for _, sharedField := range typ.SharedFields {
			if sharedField.GoName == "" { // embedded
				continue // no method needed
			}
			description := fmt.Sprintf(
				"Get%s is a part of, and documented with, the interface %s.",
				sharedField.GoName, typ.GoName)
			writeDescription(w, description)
			// In principle, we should find the corresponding field of the
			// implementation and use its name in `v.<name>`.  In practice,
			// they're always the same.
			fmt.Fprintf(w, "func (v *%s) Get%s() %s { return v.%s }\n",
				impl.Reference(), sharedField.GoName,
				sharedField.GoType.Reference(), sharedField.GoName)
		}
		fmt.Fprintf(w, "\n") // blank line between each type's implementations
	}

	// Finally, write the unmarshal-helper, which will be called by struct
	// fields referencing this type (see goStructType.WriteDefinition).
	return g.render("unmarshal_helper.go.tmpl", w, typ)
}

func (typ *goInterfaceType) Reference() string              { return typ.GoName }
func (typ *goInterfaceType) SelectionSet() ast.SelectionSet { return typ.Selection }
func (typ *goInterfaceType) GraphQLTypeName() string        { return typ.GraphQLName }

func (typ *goOpaqueType) Unwrap() goType    { return typ }
func (typ *goSliceType) Unwrap() goType     { return typ.Elem.Unwrap() }
func (typ *goPointerType) Unwrap() goType   { return typ.Elem.Unwrap() }
func (typ *goEnumType) Unwrap() goType      { return typ }
func (typ *goStructType) Unwrap() goType    { return typ }
func (typ *goInterfaceType) Unwrap() goType { return typ }

func (typ *goOpaqueType) SliceDepth() int    { return 0 }
func (typ *goSliceType) SliceDepth() int     { return typ.Elem.SliceDepth() + 1 }
func (typ *goPointerType) SliceDepth() int   { return 0 }
func (typ *goEnumType) SliceDepth() int      { return 0 }
func (typ *goStructType) SliceDepth() int    { return 0 }
func (typ *goInterfaceType) SliceDepth() int { return 0 }

func (typ *goOpaqueType) IsPointer() bool    { return false }
func (typ *goSliceType) IsPointer() bool     { return typ.Elem.IsPointer() }
func (typ *goPointerType) IsPointer() bool   { return true }
func (typ *goEnumType) IsPointer() bool      { return false }
func (typ *goStructType) IsPointer() bool    { return false }
func (typ *goInterfaceType) IsPointer() bool { return false }

func writeDescription(w io.Writer, desc string) {
	if desc != "" {
		for _, line := range strings.Split(desc, "\n") {
			fmt.Fprintf(w, "// %s\n", strings.TrimLeft(line, " \t"))
		}
	}
}
