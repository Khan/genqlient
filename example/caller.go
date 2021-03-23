package example

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/Khan/genql/graphql"
)

type authedTransport struct {
	key     string
	wrapped http.RoundTripper
}

func (t *authedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "bearer "+t.key)
	return t.wrapped.RoundTrip(req)
}

func Main() {
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

	if len(os.Args) != 2 {
		err = fmt.Errorf("usage: %v <username>", os.Args[0])
		return
	}
	username := os.Args[1]

	httpClient := http.Client{
		Transport: &authedTransport{
			key:     key,
			wrapped: http.DefaultTransport,
		},
	}
	graphqlClient := graphql.NewClient("https://api.github.com/graphql", &httpClient)

	viewerResp, err := getViewer(context.Background(), graphqlClient)
	if err != nil {
		return
	}
	fmt.Println("you are", viewerResp.Viewer.MyName)

	userResp, err := getUser(context.Background(), graphqlClient, username)
	if err != nil {
		return
	}
	fmt.Println(username, "is", userResp.User.TheirName)
}

//go:generate go run github.com/Khan/genql genql.yaml
