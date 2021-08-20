// In principle this should be "+build ignore", but for some reason
// `go mod tidy` ignores such files, so we use another build tag we never
// intend to set.

// +build tools

package testutil

import (
	// Keep golangci-lint from getting pruned from the go.mod.  We need it in
	// go.mod so that we can easily `go run` it in `make check`.
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
)
