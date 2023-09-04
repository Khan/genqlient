# example of genqlient

## Invoking the example 
Create a [personal access token](https://docs.github.com/en/github/authenticating-to-github/creating-a-personal-access-token) with no scopes.

To run the example:

```sh
$ GITHUB_TOKEN=<your token> go run .
1
2
3
4
5
```

## Running genqlient

It's already checked in to GitHub, but to regenerate `generated.go`:
```sh
go generate ./...
```

## Generating the schema file

The schema file is also checked in, but to update it, download from the [GitHub API documentation](https://docs.github.com/en/graphql/overview/public-schema).
