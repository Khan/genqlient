//go:build !go1.16
// +build !go1.16

package generate

import (
	"path/filepath"
	"runtime"
	"text/template"
)

var (
	_, thisFilename, _, _ = runtime.Caller(0)
	thisDir               = filepath.Dir(thisFilename)
)

func loadTemplate(t *template.Template, relFilename string) (*template.Template, error) {
	absFilename := filepath.Join(thisDir, relFilename)
	t, err := t.ParseFiles(absFilename)
	if err != nil {
		return t, errorf(nil, "could not load template %v from %v "+
			"(upgrade to Go 1.16 for standalone binary support): %v",
			relFilename, absFilename, err)
	}
	return t, nil
}
