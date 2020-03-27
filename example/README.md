# example of genql

## Getting a token
Get a token from [GitHub](https://github.com/settings/tokens/new) (no scopes needed).

## Invoking the example 

To run the example:

```sh
$ GITHUB_TOKEN=<your token> go run ./cmd/example/main.go <username>
you are Ben Kraft
csilvers is Craig Silverstein
```

## Running genql

It's already checked in to github, but to generate `generated.go`:
```sh
go generate ./...
```

## Generating the schema files

These are also checked in, but to update them:

```sh
npm install -g graphql-introspection-json-to-sdl
curl -H "Authorization: bearer <your token>" https://api.github.com/graphql >example/schema.json
graphql-introspection-json-to-sdl example/schema.json >example/schema.graphql
```

TODO: something better
