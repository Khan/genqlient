# genql: a truly type-safe Go GraphQL client

This is a proof-of-concept of using code-generation to create a truly type-safe GraphQL client in Go.  It is certainly not ready for production use!

## Why another GraphQL client?

To understand the issue, consider the example from the documentation of [shurcooL/graphql](https://github.com/shurcooL/graphql/):
```go
// to send a query `{ me { name } }`:
var query struct {
	Me struct {
		Name graphql.String
	}
}
// error handling omitted for brevity
client.Query(context.Background(), &query, nil)
fmt.Println(query.Me.Name)
// Output: Luke Skywalker
```
While your code is type-safe, there's nothing to check that the schema looks like you intend it to.  In fact, perhaps here we're querying the GitHub API, in which the field is called `viewer`, not `me`, so this query will fail.  Even more common than misusing the name of the field is mis-capitalizing it: the GraphQL convention is `myUrl` whereas the Go convention is `MyURL`, and it's easy to forget to use the right struct tag.  (And if you remember, it adds up to a lot of boilerplate!)  Other clients, such as [machinebox/graphql](https://github.com/machinebox/graphql), have even fewer guardrails to help you make the right query and use the result correctly.

These problems should be entirely avoidable: GraphQL and Go are both typed languages; and GraphQL servers expose their schema in a standard, machine-readable format.  We should be able to simply write a query `{ viewer { name } }`, have that automatically validated against the schema and turned into a Go struct which we can use in our code.  In fact, there's already good prior art to do this sort of thing: [99designs/gqlgen](https://github.com/99designs/gqlgen) is a popular server library that does exactly this: it generates type-safe GraphQL resolvers from a schema.

This is a proof-of-concept of a GraphQL client that does the same sort of thing: you specify the query, and it generates type-safe helpers that make your query.

## Usage

```graphql
# queries.graphql
query getViewer {
  viewer {
    MyName: name
  }
}
```

```go
// generated.go (auto-generated):
type getViewerResponse struct { ... }
func getViewer(ctx context.Context, client *graphql.Client) (*getViewerResponse, error) { ... }

// your code (error handling omitted for brevity)
graphqlClient := graphql.NewClient("https://example.com/graphql", http.DefaultClient)
viewerResp, _ := getViewer(context.Background(), graphqlClient)
fmt.Println("you are", *viewerResp.Viewer.MyName)
```

For a complete working example, see `example/`.

## Major TODOs

Query structures to support:
- repeated fields
- interfaces
- fragments

Config options:
- file locations (queries, generated, schema (or get via HTTP))
- use ctx or not, incl. complexities of how Khan uses context
- HTTP calling convention (is there enough variation to matter?)

Other:
- figure out and document go generate syntax
- error-checking/validation/etc. everywhere
- tests
- documentation
