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
fmt.Println("you are", *viewerResp.Viewer.MyName)

//go:generate go run github.com/Khan/genql
```

For a complete working example, see `example/`.

## Tests

`go test ./...` tests code generation.  (This is run by GitHub Actions.)

`make example` tests that everything wires up to a real API correctly.

TODO(benkraft): Figure out how to get GitHub Actions to run the example -- it needs a token.

## Major TODOs

Query structures to support:
- interfaces
- unions
- fragments

Config options:
- get schema via HTTP (perhaps even via GraphQL introspection)
- proper config/arguments setup (e.g. with [viper](https://github.com/spf13/viper)

Other:
- naming collisions are a mess
- error-checking/validation/etc. everywhere
- more tests
- documentation
- a name that's more clearly distinct from other libraries out there
- what to do about usages in tests -- tests will want to construct response values, and will need them to not have anonymous structs so as to do that (but that runs into even more naming collisions, if the same file has several queries with different fields of a type)

### How to support fragments and interfaces

Consider the following query (suppose that `a` returns interface type `I`, which may be implemented by either `T` or `U`):

```graphql
query { a { __typename b ...f } }
fragment f on T { c d }
```

Depending on whether the concrete type returned from `a` is `T`, we can get one of two result structures back:

```json
{"a": {"__typename": "T", "b": "...", "c": "..." , "d": "..."}}
{"a": {"__typename": "U", "b": "..."}}
```

The question is: how do we translate that to Go types?

One natural option is to generate a Go type for every concrete GraphQL type the object might have, and simply inline all the fragments.  So here we would have
```go
type T struct{ B, C, D string }
type U struct{ B string }
type I interface{ isI() }

func (t T) isI() {}
func (u U) isI() {}

type Response struct{ A I }
```

In this approach, the two objects above would look like

```go
Response{A: T{B: "...", C: "...", D: "..."}}
Response{A: U{B: "..."}}
```

Another natural option, which looks more like the way `shurcooL/graphql` does things, is to generate a type for each fragment, and only fill in the relevant ones:
```go
type Response struct {
    A struct{
        B string
        F *struct{
            C, D string
        }
    }
}
```

In this approach, the two objects above would look like

```go
Response{A: {B: "...", F: {C: "...", D: "..."}}}
Response{A: {B: "..."}}
```

(Note the types are omitted since they're not named.)

Each approach also naturally implies a different way to handle a query that uses interface types without fragments.  In particular, consider the query

```graphql
query { a { b } }
```

using the same schema as above.  In the former approach, we still define three types (plus two receivers); and `resp.A` is still of an interface type; it might be either `T` or `U`.  In the latter approach, this looks just like any other query: `resp.A` is a `struct{ B string }`.  This has implications for how we use this data: the latter approach lets us just do `resp.A.B`, whereas the former requires we do a type-switch, or add a `GetB()` method to `I`, and do `resp.A.GetB()`.


Pros of the first approach:

- it's the most natural translation of how GraphQL does things
- you always know what type you got back
- you always know which fields are there -- you don't have to encode at the application level an assumption that if fragment A was defined, fragment B also will be, because all types that match A also match B

Pros of the second approach:

- the types are simpler: in principle we don't need to define any intermediate named types
- if you don't care about the types, and don't use fragments, you don't have to worry about anything special
- we avoid having to have a bunch of interface methods (or push a type switch onto callers)
- if you're using fragments to share code, rather than to type switch, we can more naturally share that type

Note that both approaches require that we add `__typename` to every selection set which has fragments (unless the types match exactly).  This seems fine since Apollo client also does so for all selection sets.  We also need to define `UnmarshalJSON` on every type with fragment-spreads; in the former case Go doesn't know how to unmarshal into an interface type, while in the latter the Go type structure is too different from that of the JSON.  (Note that `shurcooL/graphql` actually has its own JSON-decoder to solve this problem.)

A third non-approach is to simplify define all the fields on the same struct, with some optional:

```go
type Response struct {
    A struct {
        B string
        C *string
        D *string
    }
}
```

Apart from being semantically messy, this doesn't have a good way to handle the case where there are types with conflicting fields, e.g.

```
interface I {}
type T implements I { f: Int }
type U implements I { f: String }

query {
    a {
        ... on T { f }
        ... on U { f }
    }
}
```

What type is `resp.A.F`?  It has to be both `string` and `int`.
