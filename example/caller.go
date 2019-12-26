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

	key := os.Getenv("KEY")
	if key == "" {
		err = fmt.Errorf("must set KEY=<github token>")
		return
	}

	httpClient := http.Client{
		Transport: &authedTransport{
			key:     key,
			wrapped: http.DefaultTransport,
		},
	}
	graphqlClient := graphql.NewClient("https://api.github.com/graphql", &httpClient)
	resp, err := getViewer(context.Background(), graphqlClient)
	if err != nil {
		return
	}

	fmt.Println("you are:", *resp.Viewer.Name)
}
