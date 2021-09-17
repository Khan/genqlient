package generate

import (
	"fmt"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
)

// Represents the genqlient directive, described in detail in
// docs/genqlient_directive.graphql.
type genqlientDirective struct {
	pos       *ast.Position
	Omitempty *bool
	Pointer   *bool
	Struct    *bool
	Bind      string
	TypeName  string
}

func (dir *genqlientDirective) GetOmitempty() bool { return dir.Omitempty != nil && *dir.Omitempty }
func (dir *genqlientDirective) GetPointer() bool   { return dir.Pointer != nil && *dir.Pointer }
func (dir *genqlientDirective) GetStruct() bool    { return dir.Struct != nil && *dir.Struct }

func setBool(optionName string, dst **bool, prevValue *bool, v *ast.Value) error {
	if prevValue != nil {
		return errorf(v.Position, "conflicting directives for %v", optionName)
	}
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

func setString(optionName string, dst *string, prevValue string, v *ast.Value) error {
	if prevValue != "" {
		return errorf(v.Position, "conflicting directives for %v", optionName)
	}
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

// fromGraphQL converts a parsed genqlient GraphQL directive into the
// genqlientDirective struct.
//
// If there are multiple genqlient directives are applied to the same node,
// e.g.
//	# @genqlient(...)
//	# @genqlient(...)
// fromGraphQL will be called several times, with prevDirective set to the
// result of the previous call.  In this case, conflicts between the options
// are an error.
func fromGraphQL(
	dir *ast.Directive,
	prevDirective *genqlientDirective,
	pos *ast.Position,
) (*genqlientDirective, error) {
	if dir.Name != "genqlient" {
		// Actually we just won't get here; we only get here if the line starts
		// with "# @genqlient", unless there's some sort of bug.
		return nil, errorf(pos, "the only valid comment-directive is @genqlient, got %v", dir.Name)
	}

	retval := *prevDirective
	retval.pos = pos

	var err error
	for _, arg := range dir.Arguments {
		switch arg.Name {
		// TODO(benkraft): Use reflect and struct tags?
		case "omitempty":
			err = setBool("omitempty", &retval.Omitempty, prevDirective.Omitempty, arg.Value)
		case "pointer":
			err = setBool("pointer", &retval.Pointer, prevDirective.Pointer, arg.Value)
		case "struct":
			err = setBool("struct", &retval.Struct, prevDirective.Struct, arg.Value)
		case "bind":
			err = setString("bind", &retval.Bind, prevDirective.Bind, arg.Value)
		case "typename":
			err = setString("typename", &retval.TypeName, prevDirective.TypeName, arg.Value)
		default:
			return nil, errorf(pos, "unknown argument %v for @genqlient", arg.Name)
		}
		if err != nil {
			return nil, err
		}
	}
	return &retval, nil
}

func (dir *genqlientDirective) validate(node interface{}, schema *ast.Schema) error {
	switch node := node.(type) {
	case *ast.OperationDefinition:
		if dir.Bind != "" {
			return errorf(dir.pos, "bind may not be applied to the entire operation")
		}

		// Anything else is valid on the entire operation; it will just apply
		// to whatever it is relevant to.
		return nil
	case *ast.FragmentDefinition:
		if dir.Bind != "" {
			// TODO(benkraft): Implement this if people find it useful.
			return errorf(dir.pos, "bind is not implemented for named fragments")
		}

		if dir.Struct != nil {
			return errorf(dir.pos, "struct is only applicable to fields")
		}

		// Like operations, anything else will just apply to the entire
		// fragment.
		return nil
	case *ast.VariableDefinition:
		if dir.Omitempty != nil && node.Type.NonNull {
			return errorf(dir.pos, "omitempty may only be used on optional arguments")
		}

		if dir.Struct != nil {
			return errorf(dir.pos, "struct is only applicable to fields")
		}

		return nil
	case *ast.Field:
		if dir.Omitempty != nil {
			return errorf(dir.pos, "omitempty is not applicable to fields")
		}

		if dir.Struct != nil {
			typ := schema.Types[node.Definition.Type.Name()]
			if err := validateStructOption(typ, node.SelectionSet, dir.pos); err != nil {
				return err
			}
		}

		return nil
	default:
		return errorf(dir.pos, "invalid @genqlient directive location: %T", node)
	}
}

func validateStructOption(
	typ *ast.Definition,
	selectionSet ast.SelectionSet,
	pos *ast.Position,
) error {
	if typ.Kind != ast.Interface && typ.Kind != ast.Union {
		return errorf(pos, "struct is only applicable to interface-typed fields")
	}

	// Make sure that all the requested fields apply to the interface itself
	// (not just certain implementations).
	for _, selection := range selectionSet {
		switch selection.(type) {
		case *ast.Field:
			// fields are fine.
		case *ast.InlineFragment, *ast.FragmentSpread:
			// Fragments aren't allowed. In principle we could allow them under
			// the condition that the fragment applies to the whole interface
			// (not just one implementation; and so on recursively), and for
			// fragment spreads additionally that the fragment has the same
			// option applied to it, but it seems more trouble than it's worth
			// right now.
			return errorf(pos, "struct is not allowed for types with fragments")
		}
	}
	return nil
}

// merge joins the directive applied to this node (the argument) and the one
// applied to the entire operation (the receiver) and returns a new
// directive-object representing the options to apply to this node (where in
// general we take the node's option, then the operation's, then the default).
func (dir *genqlientDirective) merge(other *genqlientDirective) *genqlientDirective {
	retval := *dir
	if other.Omitempty != nil {
		retval.Omitempty = other.Omitempty
	}
	if other.Pointer != nil {
		retval.Pointer = other.Pointer
	}
	if other.Struct != nil {
		retval.Struct = other.Struct
	}
	if other.Bind != "" {
		retval.Bind = other.Bind
	}
	// For typename, the local directive always wins: when specified on the query
	// options typename applies to the response-struct, not to all parts of the
	// query.
	retval.TypeName = other.TypeName
	return &retval
}

// parsePrecedingComment looks at the comment right before this node, and
// returns the genqlient directive applied to it (or an empty one if there is
// none), the remaining human-readable comment (or "" if there is none), and an
// error if the directive is invalid.
func (g *generator) parsePrecedingComment(
	node interface{},
	pos *ast.Position,
) (comment string, directive *genqlientDirective, err error) {
	directive = new(genqlientDirective)
	hasDirective := false
	if pos == nil || pos.Src == nil { // node was added by genqlient itself
		return "", directive, nil // treated as if there were no comment
	}

	var commentLines []string
	sourceLines := strings.Split(pos.Src.Input, "\n")
	for i := pos.Line - 1; i > 0; i-- {
		line := strings.TrimSpace(sourceLines[i-1])
		trimmed := strings.TrimSpace(strings.TrimPrefix(line, "#"))
		if strings.HasPrefix(line, "# @genqlient") {
			hasDirective = true
			var graphQLDirective *ast.Directive
			graphQLDirective, err = parseDirective(trimmed, pos)
			if err != nil {
				return "", nil, err
			}
			directive, err = fromGraphQL(graphQLDirective, directive, pos)
			if err != nil {
				return "", nil, err
			}
		} else if strings.HasPrefix(line, "#") {
			commentLines = append(commentLines, trimmed)
		} else {
			break
		}
	}

	if hasDirective { // (else directive is empty)
		err = directive.validate(node, g.schema)
		if err != nil {
			return "", nil, err
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
