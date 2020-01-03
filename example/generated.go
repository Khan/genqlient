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
`, &retval, nil)
	return &retval, err
}

type getUserResponse = struct {
	User *struct {
		Name *string
	}
}

// TODO
func getUser(ctx context.Context, client *graphql.Client, login string) (*getUserResponse, error) {
	variables := map[string]interface{}{
		"login": login,
	}

	var retval getUserResponse
	err := client.MakeRequest(ctx, `
query getUser ($login: String!) {
	User: user(login: $login) {
		Name: name
	}
}
`, &retval, variables)
	return &retval, err
}
