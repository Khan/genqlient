//go:build go1.16
// +build go1.16

package generate

import (
	"embed"
	"text/template"
)

//go:embed *.tmpl
var templateFS embed.FS

func loadTemplate(t *template.Template, relFilename string) (*template.Template, error) {
	t, err := t.ParseFS(templateFS, relFilename)
	if err != nil {
		return t, errorf(nil, "could not load template %v: %v", relFilename, err)
	}
	return t, nil
}
