package example

import (
	"context"

	"github.com/Khan/genql/graphql"
)

type getViewerResponse = struct {
	Viewer struct {
		Name *string
	}
}

// TODO
func getViewer(ctx context.Context, client *graphql.Client) (*getViewerResponse, error) {
	var retval getViewerResponse
	err := client.MakeRequest(ctx, `
query getViewer {
	Viewer: viewer {
		Name: name
	}
}
`, &retval)
	return &retval, err
}
