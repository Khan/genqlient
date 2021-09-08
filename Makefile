example:
	go generate ./...
	go run ./example/cmd/example/main.go

lint:
	( cd internal/lint && go build -o golangci-lint github.com/golangci/golangci-lint/cmd/golangci-lint)
	internal/lint/golangci-lint run ./...

check: lint
	go test -cover ./...
	go mod tidy

genqlient.png: genqlient.svg
	convert -density 600 -background transparent "$<" "$@"

.PHONY: example
