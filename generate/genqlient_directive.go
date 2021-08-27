package generate

import (
	"fmt"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
)

// GenqlientDirective represents the @genqlient quasi-directive, used to
// configure genqlient on a query-by-query basis.
//
// The syntax of the directive is just like a GraphQL directive, except it goes
// in a comment on the line immediately preceding the field.  (This is because
// GraphQL expects directives in queries to be defined by the server, not by
// the client, so it would reject a real @genqlient directive as nonexistent.)
//
// Directives may be applied to fields, arguments, or the entire query.
// Directives on the line preceding the query apply to all relevant nodes in
// the query; other directives apply to all nodes on the following line.  (In
// all cases it's fine for there to be other comments in between the directive
// and the node(s) to which it applies.)  For example, in the following query:
//	# @genqlient(n: "a")
//
//	# @genqlient(n: "b")
//	#
//	# Comment describing the query
//  #
//	# @genqlient(n: "c")
//	query MyQuery(arg1: String,
//		# @genqlient(n: "d")
//		arg2: String, arg3: String,
//		arg4: String,
//	) {
//		# @genqlient(n: "e")
//		field1, field2
//		field3
//	}
// the directive "a" is ignored, "b" and "c" apply to all relevant nodes in the
// query, "d" applies to arg2 and arg3, and "e" applies to field1 and field2.
type GenqlientDirective struct {
	pos *ast.Position

	// If set, this argument will be omitted if it's equal to its Go zero
	// value, or is an empty slice.
	//
	// For example, given the following query:
	//	# @genqlient(omitempty: true)
	//	query MyQuery(arg: String) { ... }
	// genqlient will generate a function
	//	MyQuery(ctx context.Context, client graphql.Client, arg string) ...
	// which will pass {"arg": null} to GraphQL if arg is "", and the actual
	// value otherwise.
	//
	// Only applicable to arguments of nullable types.
	Omitempty *bool

	// If set, this argument or field will use a pointer type in Go.  Response
	// types always use pointers, but otherwise we typically do not.
	//
	// This can be useful if it's a type you'll need to pass around (and want a
	// pointer to save copies) or if you wish to distinguish between the Go
	// zero value and null (for nullable fields).
	Pointer *bool

	// If set, this argument or field will use the given Go type instead of a
	// genqlient-generated type.
	//
	// The value should be the fully-qualified type name, or e.g.
	// []path/to/my.Type if the GraphQL type is [MyType].  (This allows
	// binding a GraphQL list type to a Go struct type, if you want.)
	//
	// See Config.Bindings for more details; this is effectively to a local
	// version of that global setting and should be used with similar care.
	// If set to "-", overrides any such global setting and uses a
	// genqlient-generated type.
	Bind string
}

func (dir *GenqlientDirective) GetOmitempty() bool { return dir.Omitempty != nil && *dir.Omitempty }
func (dir *GenqlientDirective) GetPointer() bool   { return dir.Pointer != nil && *dir.Pointer }

func setBool(dst **bool, v *ast.Value) error {
	ei, err := v.Value(nil) // no vars allowed
	if err != nil {
		return errorf(v.Position, "invalid boolean value %v: %v", v, err)
	}
	if b, ok := ei.(bool); ok {
		*dst = &b
		return nil
	}
	return errorf(v.Position, "expected boolean, got non-boolean value %T(%v)", ei, ei)
}

func setString(dst *string, v *ast.Value) error {
	ei, err := v.Value(nil) // no vars allowed
	if err != nil {
		return errorf(v.Position, "invalid string value %v: %v", v, err)
	}
	if b, ok := ei.(string); ok {
		*dst = b
		return nil
	}
	return errorf(v.Position, "expected string, got non-string value %T(%v)", ei, ei)
}

func fromGraphQL(dir *ast.Directive) (*GenqlientDirective, error) {
	if dir.Name != "genqlient" {
		// Actually we just won't get here; we only get here if the line starts
		// with "# @genqlient", unless there's some sort of bug.
		return nil, errorf(dir.Position, "the only valid comment-directive is @genqlient, got %v", dir.Name)
	}

	var retval GenqlientDirective
	retval.pos = dir.Position

	var err error
	for _, arg := range dir.Arguments {
		switch arg.Name {
		// TODO: reflect and struct tags?
		case "omitempty":
			err = setBool(&retval.Omitempty, arg.Value)
		case "pointer":
			err = setBool(&retval.Pointer, arg.Value)
		case "bind":
			err = setString(&retval.Bind, arg.Value)
		default:
			return nil, errorf(arg.Position, "unknown argument %v for @genqlient", arg.Name)
		}
		if err != nil {
			return nil, err
		}
	}
	return &retval, nil
}

func (dir *GenqlientDirective) validate(node interface{}) error {
	switch node := node.(type) {
	case *ast.OperationDefinition:
		if dir.Bind != "" {
			return errorf(dir.pos, "bind may not be applied to the entire operation")
		}

		// Anything else is valid on the entire operation; it will just apply
		// to whatever it is relevant to.
		return nil
	case *ast.VariableDefinition:
		if dir.Omitempty != nil && node.Type.NonNull {
			return errorf(dir.pos, "omitempty may only be used on optional arguments")
		}
		return nil
	case *ast.Field:
		if dir.Omitempty != nil {
			return errorf(dir.pos, "omitempty is not applicable to fields")
		}
		return nil
	default:
		return errorf(dir.pos, "invalid directive location: %T", node)
	}
}

func (dir *GenqlientDirective) merge(other *GenqlientDirective) *GenqlientDirective {
	retval := *dir
	if other.Omitempty != nil {
		retval.Omitempty = other.Omitempty
	}
	if other.Pointer != nil {
		retval.Pointer = other.Pointer
	}
	if other.Bind != "" {
		retval.Bind = other.Bind
	}
	return &retval
}

func (g *generator) parsePrecedingComment(
	node interface{},
	pos *ast.Position,
) (comment string, directive *GenqlientDirective, err error) {
	directive = new(GenqlientDirective)
	if pos == nil || pos.Src == nil { // node was added by genqlient itself
		return "", directive, nil // treated as if there were no comment
	}

	var commentLines []string
	sourceLines := strings.Split(pos.Src.Input, "\n")
	for i := pos.Line - 1; i > 0; i-- {
		line := strings.TrimSpace(sourceLines[i-1])
		trimmed := strings.TrimSpace(strings.TrimPrefix(line, "#"))
		if strings.HasPrefix(line, "# @genqlient") {
			graphQLDirective, err := parseDirective(trimmed, pos)
			if err != nil {
				return "", nil, err
			}
			genqlientDirective, err := fromGraphQL(graphQLDirective)
			if err != nil {
				return "", nil, err
			}
			err = genqlientDirective.validate(node)
			if err != nil {
				return "", nil, err
			}
			directive = directive.merge(genqlientDirective)
		} else if strings.HasPrefix(line, "#") {
			commentLines = append(commentLines, trimmed)
		} else {
			break
		}
	}

	reverse(commentLines)

	return strings.TrimSpace(strings.Join(commentLines, "\n")), directive, nil
}

func parseDirective(line string, pos *ast.Position) (*ast.Directive, error) {
	// HACK: parse the "directive" by making a fake query containing it.
	fakeQuery := fmt.Sprintf("query %v { field }", line)
	doc, err := parser.ParseQuery(&ast.Source{Input: fakeQuery})
	if err != nil {
		return nil, errorf(pos, "invalid genqlient directive: %v", err)
	}
	return doc.Operations[0].Directives[0], nil
}
