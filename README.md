![generated graphql client ⇒ genqlient](genqlient.png)

# genqlient: a truly type-safe Go GraphQL client

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

//go:generate go run github.com/Khan/genqlient
```

For a complete working example, see `example/`.  For configuration options, see `go doc github.com/Khan/genqlient/generate.Config`.

TODO: document this a bit more, including different ways to specify queries, options once we have those, etc.

## Documentation for generated code

For each GraphQL operation (query or mutation), genqlient generates a Go function with the exact same name, which accepts:
- a `context.Context` (unless configured otherwise)
- a `genqlient/graphql.Client` (you might have a global one, or init it inline)
- arguments corresponding to the query arguments

It returns a pointer to a struct representing the query-result, and an `error`.  The struct will always be initialized (never nil), even on error.  The error may be a `github.com/vektah/gqlparser/v2/gqlerror.List`, if it was a GraphQL-level error (in this case the returned struct may still contain useful data, if the API returns data even on error), or may be another error if, for example, the whole HTTP request failed (in which case the struct is unlikely to contain useful data).  If the GraphQL operation has a comment immediately above it, that comment text will be used as the GoDoc for the generated function.

TODO: document generated types further, especially if they become customizable.

## Tests

`go test ./...` tests code generation.  (This is run by GitHub Actions.)  Most of the tests are snapshot-based; see `generate/generate_test.go`.

`make example` rebuilds the example, and tests that everything wires up to a real API correctly.  This is not currently included in `go test`, since it requires a token.

TODO(benkraft): Figure out how to get GitHub Actions a token to run the example.

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
- proper config/arguments setup (e.g. with [viper](https://github.com/spf13/viper))
- (+) improve client_getter to be more usable (and document it), or flag it out for now.
- get schema via GraphQL introspection (although honestly, it seems like SDL is becoming popular enough there may be no need)
- send hash rather than full query
- whether names should be exported
- default handling for optional fields (pointers, HasFoo, etc.)
- response/function-name format (e.g. force exported/unexported, change "Response" suffix, change how input objects work, etc.)
- generate mocks?

Other:
- (+) improved validation and error checking
- (+) documentation
- get a designer to fix my bad logo-thing
- custom scalar types (or custom mappings for standard scalars, if you want a special ID type say)
- allow mapping a GraphQL type to a particular Go type (if you want to use a named type for some string, say)
- (optionally?) include full query in generated godoc
