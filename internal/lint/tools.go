// Conventionally this sort of thing would use the "ignore" tag, but
// `go mod tidy` ignores so-tagged files explicitly, so we use another build
// tag we never intend to set.
// +build tools
//go:build tools

// Package lint exists to pin a version of golangci-lint, but to keep it out of
// our main go.mod.  This is useful because end-users typically want to pin
// their own version of golangci-lint (since any new lint check may fail on
// their codebase) but go mod doesn't really like doing that.  Luckily, we only
// need it for lint, so we just put it in a separate module.
package lint

import _ "github.com/golangci/golangci-lint/cmd/golangci-lint"
