# Getting started with genqlient

This document describes how to set up genqlient and use it for simple queries.  See also the full worked [example](../example), the [FAQ](FAQ.md), and the reference for [project-wide](genqlient.yaml) and [query-specific](genqlient_directive.graphql) configuration options.

## Step 1: Download your schema

You want the schema in GraphQL [Schema Definition Language (SDL)](https://graphql.org/learn/schema/#type-language) format.  For example, to query the GitHub API, you could download the schema from [their documentation](https://docs.github.com/en/graphql/overview/public-schema).  Put this in `schema.graphql`.

## Step 2: Write your queries

Next, write your GraphQL query.  This is often easiest to do in an interactive explorer like [GraphiQL](https://github.com/graphql/graphiql/tree/main/packages/graphiql#readme); the syntax is just standard [GraphQL syntax](https://graphql.org/learn/queries/) and supports queries, mutations and subscriptions.  Put it in `genqlient.graphql`:
```graphql
query getUser($login: String!) {
  user(login: $login) {
    name
  }
}
```

## Step 3: Run genqlient

Now, run `go run github.com/Khan/genqlient --init`.  This will create a configuration file, and then run genqlient to produce a file `generated.go` with your queries.

## Step 4: Use your queries

Finally, write your code!  The generated code will expose a function with the same name as your query, here
```go
func getUser(ctx context.Context, client graphql.Client, login string) (*getUserResponse, error)
```

As for the arguments:
- for `ctx`, pass your local context (see [`go doc context`](https://pkg.go.dev/context)) or `context.Background()` if you don't need one
- for `client`, call [`graphql.NewClient`](https://pkg.go.dev/github.com/Khan/genqlient/graphql), e.g. `graphql.NewClient("https://your.api.example/path", http.DefaultClient)`
- for `login`, pass your GitHub username (or whatever the arguments to your query are)

The response object is a struct with fields corresponding to each GraphQL field; for the exact details check its GoDoc (perhaps via your IDE's autocomplete or hover).  For example, you might do:
```go
ctx := context.Background()
client := graphql.NewClient("https://api.github.com/graphql", http.DefaultClient)
resp, err := getUser(ctx, client, "benjaminjkraft")
fmt.Println(resp.User.Name, err)
```

Now run your code!

## Step 5: Repeat

Over time, as you add or change queries, you'll just need to run `github.com/Khan/genqlient` to re-generate `generated.go`.  (Or add a line `//go:generate go run github.com/Khan/genqlient` in your source, and run [`go generate`](https://go.dev/blog/generate).)  If you're using an editor or IDE plugin backed by [gopls](https://github.com/golang/tools/blob/master/gopls/README.md) (which is most of them), keep `generated.go` open in the background, and reload it after each run, so your plugin knows about the automated changes.

If you prefer, you can specify your queries as string-constants in your Go source, prefixed with `# @genqlient` -- at Khan we put them right next to the calling code, e.g.
```go
_ = `# @genqlient
  query getUser($login: String!) {
    user(login: $login) {
      name
    }
  }
`

resp, err := getUser(...)
```
(You don't need to do anything with the constant, just keep it somewhere in the source as documentation and for the next time you run genqlient.)  In this case you'll need to update `genqlient.yaml` to tell it to look at your Go code.

All the filenames above, and many other aspects of genqlient, are configurable; see [genqlient.yaml](genqlient.yaml) for the full range of options.  You can also configure how genqlient converts specific parts of your query with the [`@genqlient` directive](genqlient_directive.graphql).  See the [FAQ](FAQ.md) for common options.

If you want to know even more, and help contribute to genqlient, see [DESIGN.md](DESIGN.md) and [CONTRIBUTING.md](CONTRIBUTING.md).  Happy querying!
