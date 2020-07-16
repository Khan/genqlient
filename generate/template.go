package generate

import (
	"path/filepath"
	"runtime"
	"text/template"
)

// TODO: package templates into the binary using one of those asset thingies
var _, thisFilename, _, _ = runtime.Caller(0)
var thisDir = filepath.Dir(thisFilename)

func mustTemplate(relFilename string) *template.Template {
	return template.Must(template.ParseFiles(filepath.Join(thisDir, relFilename)))
}
