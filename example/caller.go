package example

import (
	"context"
	"fmt"
	"os"
)

func Main() {
	resp, err := getViewer(context.Background())
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("you are:", resp.Viewer.Name)
}
