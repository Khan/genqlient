example:
	go generate ./...
	go run ./example

lint:
	( cd internal/lint && go build -o golangci-lint github.com/golangci/golangci-lint/cmd/golangci-lint )
	internal/lint/golangci-lint run ./...

check: lint
	go test -cover ./...
	go mod tidy

.PHONY: example
