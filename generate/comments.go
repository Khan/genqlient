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
	// If set, this argument will be omitted if it's equal to its Go zero
	// value.  For example, given the following query:
	//	# @genqlient(omitempty: true)
	//	query MyQuery(arg: String) { ... }
	// genqlient will generate a function
	//	MyQuery(ctx context.Context, client graphql.Client, arg string) ...
	// which will pass {"arg": null} to GraphQL if arg is "", and the actual
	// value otherwise.
	//
	// Only applicable to arguments of nullable types.
	Omitempty bool
}

func fromGraphQL(dir *ast.Directive) (*GenqlientDirective, error) {
	if dir.Name != "genqlient" {
		// Actually we just won't get here; we only get here if the line starts
		// with "# @genqlient", unless there's some sort of bug.
		return nil, fmt.Errorf("the only valid comment-directive is @genqlient, got %v", dir.Name)
	}

	var retval GenqlientDirective
	for _, arg := range dir.Arguments {
		switch arg.Name {
		case "omitempty":
			retval.Omitempty = true
		default:
			return nil, fmt.Errorf("unknown argument %v for @genqlient", arg.Name)
		}
	}
	return &retval, nil
}

func (dir *GenqlientDirective) validate(node interface{}) error {
	switch node := node.(type) {
	case *ast.OperationDefinition:
		// Anything is valid on the entire operation; it will just apply to
		// whatever it is relevant to.
		return nil
	case *ast.VariableDefinition:
		if dir.Omitempty && node.Type.NonNull {
			return fmt.Errorf("omitempty may only be used on optional arguments")
		}
		return nil
	case *ast.Field:
		if dir.Omitempty {
			return fmt.Errorf("omitempty is not appilcable to fields")
		}
		return nil
	default:
		return fmt.Errorf("invalid directive location: %T", node)
	}
}

func (dir *GenqlientDirective) merge(other *GenqlientDirective) *GenqlientDirective {
	if dir == nil {
		return other
	}
	var retval GenqlientDirective
	retval.Omitempty = dir.Omitempty || other.Omitempty
	return &retval
}

func (g *generator) parsePrecedingComment(
	node interface{},
	pos *ast.Position,
) (comment string, directive *GenqlientDirective, err error) {
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
	doc, err := parser.ParseQuery(&ast.Source{
		Name: fmt.Sprintf("@genqlient directive at %v:%v:%v",
			pos.Src.Name, pos.Line, pos.Column),
		Input: fakeQuery,
	})
	if err != nil {
		return nil, err
	}
	return doc.Operations[0].Directives[0], nil
}