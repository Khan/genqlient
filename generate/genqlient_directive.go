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
}

func (dir *genqlientDirective) GetOmitempty() bool { return dir.Omitempty != nil && *dir.Omitempty }
func (dir *genqlientDirective) GetPointer() bool   { return dir.Pointer != nil && *dir.Pointer }
func (dir *genqlientDirective) GetStruct() bool    { return dir.Struct != nil && *dir.Struct }

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

func fromGraphQL(dir *ast.Directive, pos *ast.Position) (*genqlientDirective, error) {
	if dir.Name != "genqlient" {
		// Actually we just won't get here; we only get here if the line starts
		// with "# @genqlient", unless there's some sort of bug.
		return nil, errorf(pos, "the only valid comment-directive is @genqlient, got %v", dir.Name)
	}

	var retval genqlientDirective
	retval.pos = pos

	var err error
	for _, arg := range dir.Arguments {
		switch arg.Name {
		// TODO: reflect and struct tags?
		case "omitempty":
			err = setBool(&retval.Omitempty, arg.Value)
		case "pointer":
			err = setBool(&retval.Pointer, arg.Value)
		case "struct":
			err = setBool(&retval.Struct, arg.Value)
		case "bind":
			err = setString(&retval.Bind, arg.Value)
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
	return &retval
}

func (g *generator) parsePrecedingComment(
	node interface{},
	pos *ast.Position,
) (comment string, directive *genqlientDirective, err error) {
	directive = new(genqlientDirective)
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
			genqlientDirective, err := fromGraphQL(graphQLDirective, pos)
			if err != nil {
				return "", nil, err
			}
			err = genqlientDirective.validate(node, g.schema)
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
