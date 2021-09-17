package generate

import (
	"io"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
)

var (
	// TODO(benkraft): Embed templates into the binary, see
	// https://github.com/Khan/genqlient/issues/9.
	_, thisFilename, _, _ = runtime.Caller(0)
	thisDir               = filepath.Dir(thisFilename)
)

func repeat(n int, s string) string {
	var builder strings.Builder
	for i := 0; i < n; i++ {
		builder.WriteString(s)
	}
	return builder.String()
}

func intRange(n int) []int {
	ret := make([]int, n)
	for i := 0; i < n; i++ {
		ret[i] = i
	}
	return ret
}

func sub(x, y int) int { return x - y }

// render executes the given template with the funcs from this generator.
func (g *generator) render(tmplRelFilename string, w io.Writer, data interface{}) error {
	tmpl := g.templateCache[tmplRelFilename]
	if tmpl == nil {
		absFilename := filepath.Join(thisDir, tmplRelFilename)
		funcMap := template.FuncMap{
			"ref":      g.ref,
			"repeat":   repeat,
			"intRange": intRange,
			"sub":      sub,
		}
		var err error
		tmpl, err = template.New(tmplRelFilename).Funcs(funcMap).ParseFiles(absFilename)
		if err != nil {
			return errorf(nil, "could not load template %v: %v", absFilename, err)
		}
		g.templateCache[tmplRelFilename] = tmpl
	}
	err := tmpl.Execute(w, data)
	if err != nil {
		return errorf(nil, "could not render template: %v", err)
	}
	return nil
}
