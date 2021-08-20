// Conventionally this sort of thing would use the "ignore" tag, but
// `go mod tidy` ignores so-tagged files explicitly, so we use another build
// tag we never intend to set.
// +build tools
//go:build tools

package testutil

import (
	// Keep golangci-lint from getting pruned from the go.mod.  We need it in
	// go.mod so that we can easily `go run` it in `make check`.
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
)
