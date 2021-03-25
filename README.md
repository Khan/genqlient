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
While your code is in principle type-safe, there's nothing to check that the schema looks like you expect it to.  In fact, perhaps here we're querying the GitHub API, in which the field is called `viewer`, not `me`, so this query will fail.  Even more common than misusing the name of the field is mis-capitalizing it: the GraphQL convention is `myUrl` whereas the Go convention is `MyURL`, and it's easy to forget or mistype the struct tag.  (Even if you get it right, it adds up to a lot of handwritten boilerplate!)  Other clients, such as [machinebox/graphql](https://github.com/machinebox/graphql), have even fewer guardrails to help you make the right query and use the result correctly.  This isn't a big deal in a small application, but for serious production-grade tools it's not ideal.

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
fmt.Println("you are", viewerResp.Viewer.MyName)

//go:generate go run github.com/Khan/genql
```

For a complete working example, see `example/`.

## Tests

`go test ./...` tests code generation.  (This is run by GitHub Actions.)

Most of the tests are snapshot-based; they use the schema, queries, and snapshots in `generate/testdata`.  The schema is in `schema.graphql`; the queries are in `TestName.graphql`.  The test by default asserts that the output of the generator matches the snapshot `TestName.graphql.go`.  To update the snapshots, run with `UPDATE_SNAPSHOTS=1`; it will fail the tests and print the diffs, but regenerate the snapshots.  Make sure to check that the output is sensible!

`make example` rebuilds the example, and tests that everything wires up to a real API correctly.

TODO(benkraft): Figure out how to get GitHub Actions to run the example -- it needs a token.

## Design

See [DESIGN.md](DESIGN.md) for documentation of major design decisions in this library.

## Major TODOs

(*) denotes things we need to use this in prod at Khan
(+) denotes things we further need before recommending anyone else use this in prod

Generated code:
- add flag(s) to make a field use a pointer (for optionality or perf; see DESIGN)
- redo support for interfaces, unions, fragments (see DESIGN)
- (optional) collapsing -- should be able to have `mutation { myMutation { error { code } } }` just return `(code string, err error)`

Config options:
- (+) proper config/arguments setup (e.g. with [viper](https://github.com/spf13/viper))
- (+) improve client_getter to be more usable (and document it), or flag it out for now.
- get schema via HTTP (perhaps even via GraphQL introspection)
- send hash rather than full query
- whether names should be exported
- default handling for optional fields (pointers, HasFoo, etc.)
- response/function-name format (e.g. force exported/unexported, change "Response" suffix, change how input objects work, etc.)
- generate mocks?

Other:
- (*) error-checking/validation/etc. everywhere
- (*) a name that's more clearly distinct from other libraries out there and conveys what this does
- (+) more tests
- (+) documentation
- custom scalar types (or custom mappings for standard scalars, if you want a special ID type say)
- allow mapping a custom type to a particular val (if you want to use a named type for some string, say)
