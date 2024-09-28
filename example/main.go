package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/Khan/genqlient/graphql"
)

type authedTransport struct {
	key     string
	wrapped http.RoundTripper
}

func (t *authedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "bearer "+t.key)
	return t.wrapped.RoundTrip(req)
}

func main() {
	var err error
	defer func() {
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}()

	key := os.Getenv("GITHUB_TOKEN")
	if key == "" {
		err = fmt.Errorf("must set GITHUB_TOKEN=<github token>")
		return
	}

	httpClient := http.Client{
		Transport: &authedTransport{
			key:     key,
			wrapped: http.DefaultTransport,
		},
	}
	graphqlClient := graphql.NewClient("https://api.github.com/graphql", &httpClient)

	switch len(os.Args) {
	case 1:
		var viewerResp *getViewerResponse
		viewerResp, err = getViewer(context.Background(), graphqlClient)
		if err != nil {
			return
		}
		fmt.Println("you are", viewerResp.Viewer.MyName, "created on", viewerResp.Viewer.CreatedAt.Format("2006-01-02"))

	case 2:
		username := os.Args[1]
		var userResp *getUserResponse
		userResp, err = getUser(context.Background(), graphqlClient, username)
		if err != nil {
			return
		}
		fmt.Println(username, "is", userResp.User.TheirName, "created on", userResp.User.CreatedAt.Format("2006-01-02"))

	default:
		err = fmt.Errorf("usage: %v [username]", os.Args[0])
	}
}

//go:generate go run github.com/Khan/genqlient genqlient.yaml
