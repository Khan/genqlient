package generate

import (
	"fmt"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
)

func (g *generator) parsePrecedingComment(pos *ast.Position) (comment string, directives []*ast.Directive, err error) {
	var commentLines []string
	sourceLines := strings.Split(pos.Src.Input, "\n")
	for i := pos.Line - 1; i > 0; i-- {
		line := strings.TrimSpace(sourceLines[i-1])
		trimmed := strings.TrimSpace(strings.TrimPrefix(line, "#"))
		if strings.HasPrefix(line, "# @genqlient") {
			directive, err := parseDirective(trimmed, pos)
			if err != nil {
				return "", nil, err
			}
			directives = append(directives, directive)
		} else if strings.HasPrefix(line, "#") {
			commentLines = append(commentLines, trimmed)
		} else {
			break
		}
	}

	reverse(commentLines)

	return strings.TrimSpace(strings.Join(commentLines, "\n")), directives, nil
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
