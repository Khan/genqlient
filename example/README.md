# example of genqlient

## Getting a token
Get a token from [GitHub](https://github.com/settings/tokens/new) (no scopes needed).

## Invoking the example 

To run the example:

```sh
$ GITHUB_TOKEN=<your token> go run ./cmd/example/main.go <username>
you are Ben Kraft
csilvers is Craig Silverstein
```

## Running genqlient

It's already checked in to github, but to generate `generated.go`:
```sh
go generate ./...
```

## Generating the schema file

The schema file is also checked in, but to update it, download from the [GitHub API documentation](https://docs.github.com/en/graphql/overview/public-schema).
