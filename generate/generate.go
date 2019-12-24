package generate

import (
	"fmt"
	"os"
	"text/template"

	"github.com/vektah/gqlparser"
)

var _ = gqlparser.LoadSchema

// TODO: package template into the binary using one of those asset thingies
const tmplFilename = "generate/operation.go.tmpl"

type TemplateParams struct {
	// The name of the package into which to generate the operation-helpers.
	PackageName string
	// The type-name for the operation's response type.
	ResponseName string
	// The body of the operation's response type (e.g. struct { ... }).
	ResponseType string
	// The documentation for the operation, from GraphQL.
	OperationDoc string
	// The name of the operation, from GraphQL.
	OperationName string
	// The endpoint to which to send queries.
	Endpoint string
	// The body of the operation to send.
	Operation string
}

var tmpl = template.Must(template.ParseFiles(tmplFilename))

func Generate() error {
	var data TemplateParams
	err := tmpl.Execute(os.Stdout, data)
	if err != nil {
		return fmt.Errorf("template did not render: %v", err)
	}
	return nil
}
