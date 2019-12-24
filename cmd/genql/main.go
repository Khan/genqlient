package main

import (
	"fmt"
	"os"

	"github.com/Khan/genql/generate"
)

func main() {
	err := generate.Generate()
	if err != nil {
		fmt.Println(fmt.Errorf("genql failed: %v", err))
		os.Exit(1)
	}
}
