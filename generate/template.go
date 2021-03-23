package generate

import (
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

func mustTemplate(relFilename string) *template.Template {
	return template.Must(template.ParseFiles(filepath.Join(thisDir, relFilename)))
}
