package example

import (
	"context"
	"fmt"
	"os"

	"github.com/Khan/genql/graphql"
)

func Main() {
	client := graphql.NewClient("https://api.github.com/graphql", nil)
	resp, err := getViewer(context.Background(), client)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("you are:", resp.Viewer.Name)
}
