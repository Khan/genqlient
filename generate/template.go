package generate

import (
	"io"
	"path/filepath"
	"runtime"
	"text/template"
)

// TODO: package templates into the binary using one of those asset thingies
// (e.g. embed, if we wait until 1.16 to do this)
var (
	_, thisFilename, _, _ = runtime.Caller(0)
	thisDir               = filepath.Dir(thisFilename)
)

// execute executes the given template with the funcs from this generator.
func (g *generator) execute(tmplRelFilename string, w io.Writer, data interface{}) error {
	tmpl := g.templateCache[tmplRelFilename]
	if tmpl == nil {
		absFilename := filepath.Join(thisDir, tmplRelFilename)
		funcMap := template.FuncMap{
			"ref": g.ref,
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
