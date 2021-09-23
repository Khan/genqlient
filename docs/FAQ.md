# Frequently Asked Questions

This document describes common questions about genqlient, and provides an index to how to represent common query structures.  For a full list of configuration options, see [genqlient.yaml](genqlient.yaml) and [genqlient_directive.graphql](genqlient_directive.graphql).

## How do I set up genqlient to …

### … get started?

There's a [doc for that](INTRODUCTION.md)!

### … use an API that requires authentication?

When you call `graphql.NewClient`, pass in an HTTP client that adds whatever authentication headers you need (typically by wrapping the client's `Transport`).  For example:

```go
type authedTransport struct {
  wrapped http.RoundTripper
}

func (t *authedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
  key := ...
  req.Header.Set("Authorization", "bearer "+key)
  return t.wrapped.RoundTrip(req)
}

func MakeQuery(...) {
  client := graphql.NewClient("https://api.github.com/graphql",
    &http.Client{Transport: &authedTransport{wrapped: http.DefaultTransport}})

  resp, err := MyQuery(ctx, client, ...)
}
```

For more on wrapping HTTP clients, see [this post](https://dev.to/stevenacoffman/tripperwares-http-client-middleware-chaining-roundtrippers-3o00).

### … make requests against a mock server, for tests?

Testing code that uses genqlient typically involves passing in a special HTTP client that does what you want, similar to authentication.  For example, you might write a client whose `RoundTrip` returns a fixed response, constructed with [`httptest`](https://pkg.go.dev/net/http/httptest).  Or, you can use `httptest` to start up a temporary server, and point genqlient at that.  Many third-party packages provide support for this sort of thing; genqlient should work with any HTTP-level mocking that can expose a regular `http.Client`.

### … test my GraphQL APIs?

If you want, you can use genqlient to test your GraphQL APIs; as with mocking you can point genqlient at anything that exposes an ordinary HTTP endpoint or a custom `http.Client`.  However, at Khan Academy we've found that genqlient usually isn't the best client for testing; we prefer to use a lightweight (and weakly-typed) client for that, and may separately open-source ours in the future.

### … handle GraphQL errors?

Each genqlient-generated helper function returns two results, a pointer to a response-struct, and an error.  The response-struct will always be initialized (never nil), even on error.  If the request returns a valid GraphQL response containing errors, the returned error will be [`As`-able](https://pkg.go.dev/errors#As) as [`gqlerror.List`](https://pkg.go.dev/github.com/vektah/gqlparser/v2/gqlerror#List), and the struct may be partly-populated (if one field failed but another was computed successfully).  If the request fails entirely, the error will be another error (e.g. a [`*url.Error`](https://pkg.go.dev/net/url#Error)), and the response will be blank (but still non-nil).

For example, you might do one of the following:
```go
// return both error and field:
resp, err := GetUser(...)
return resp.User.Name, err

// handle different errors differently:
resp, err := GetUser(...)
var errList *gqlerror.List
if errors.As(err, &errList) {
  for _, err := range errList {
    fmt.Printf("%v at %v\n", err.Message, err.Path)
  }
  fmt.Printf("partial response: %v\n", resp)
} else if err != nil {
  fmt.Printf("http/network error: %v\n", err)
} else {
  fmt.Printf("successful response: %v\n", resp)
}
```

### … use custom scalars?

Just tell genqlient via the `bindings` option in `genqlient.yaml`:

```yaml
bindings:
  DateTime:
    type: time.Time
```

Make sure the given type has whatever logic is needed to convert to/from JSON (e.g. `MarshalJSON`/`UnmarshalJSON` or JSON tags).  See the [`genqlient.yaml` documentation](genqlient.yaml) for the full syntax.

### … require 32-bit integers?

The GraphQL spec officially defines the `Int` type to be a [signed 32-bit integer](https://spec.graphql.org/draft/#sec-Int).  GraphQL clients and servers vary wildly in their enforcement of this; for example:
- [Apollo Server](https://github.com/apollographql/apollo-server/) explicitly checks that integers are at most 32 bits
- [gqlgen](https://github.com/99designs/gqlgen) by default allows any integer that fits in `int` (i.e. 64 bits on most platforms)
- [Apollo Client](https://github.com/apollographql/apollo-client) doesn't check (but implicitly is limited to 53 bits by JavaScript)
- [shurcooL/graphql](https://github.com/shurcooL/graphql) requires integers be passed as a `graphql.Int`, defined to be an `int32`

By default, genqlient maps GraphQL `Int`s to Go's `int`, meaning that on 64 bit systems there's no client-side restriction.  If you prefer to limit integers to `int32`, you can set a binding in your `genqlient.yaml`:

```yaml
bindings:
  Int:
    type: int32
```

Or, you can bind it to any other type, perhaps one with size-checked constructors; see the [`genqlient.yaml` documentation](`genqlient.yaml`) for more details.

### … let me json-marshal my response objects

This is supported by default!  All genqlient-generated types support both JSON-marshaling and unmarshaling, which can be useful for putting them in a cache, inspecting them by hand, using them in mocks (although this is [not recommended](#-test-my-graphql-apis)), or anything else you can do with JSON.  It's not guaranteed that marshaling a genqlient type will produce the exact GraphQL input -- we try to get as close as we can but there are some limitations around Go zero values -- but unmarshaling again should produce the value genqlient returned.  That is:

```go
resp, err := MyQuery(...)
// not guaranteed to match what the server sent (but close):
b, err := json.Marshal(resp)
// guaranteed to match resp:
var respAgain MyQueryResponse
err := json.Unmarshal(b, &resp)
```

## How do I make a query with …

### … a specific name for a field?

genqlient supports GraphQL field-aliases, and uses them to determine the Go struct field name.  For example, if you do
```graphql
query MyQuery {
  myGreatName: myString
}
```
and genqlient will generate a Go field `MyGreatName string`.  Note that the alias will always be uppercased, to ensure the field is visible to the Go JSON library.

### … nullable fields?

There are two ways to handle nullable fields in genqlient.  One way is to use the Go idiom, where null gets mapped to the zero value; this is the default in genqlient.  So if you have a GraphQL field of type `String`, and you do:

```graphql
query MyQuery(arg: String) {
  myString
}
```

then genqlient will generate a Go field `MyString string`, and set it to the empty string if the server returns null.  This works even for structs: if an object type in GraphQL is null, genqlient will set the corresponding struct to its zero value.  It can be helpful to request `id` in such cases, since that’s a field that should always be set, or `__typename` which is guaranteed to be set, so you can use its presence to decide whether to look at the other fields.

For input fields, you often want to tell genqlient to send null to the server if the argument is set to the zero value, similar to the JSON `omitempty` tag.  In this case, you can do:

```graphql
query MyQuery(
  # @genqlient(omitempty: true)
  arg: String,
) {
  myString
}
```

You can also put the `# @genqlient(omitempty: true)` on the first line, which will apply it to all arguments in the query.

If you need to distinguish null from the empty string (or generally from the Go zero value of your type), you can tell genqlient to use a pointer for the field or argument like this:
```graphql
query MyQuery(
  # @genqlient(pointer: true)
  arg: String,
) {
  # @genqlient(pointer: true)
  myString
}
```

This will generate a Go field `MyString *string`, and set it to `nil` if the server returns null (and in reverse for arguments).  Such fields can be harder to work with in Go, but allow a clear distinction between null and the Go zero value.  Again, you can put the directive on the first line to apply it to everything in the query, although this usually gets cumbersome.

See [genqlient_directive.graphql](genqlient_directive.graphql) for complete documentation on these options.

### … GraphQL interfaces?

If you request an interface field, genqlient generates an interface type corresponding to the GraphQL interface, and several struct types corresponding to its implementations.  For example, given a query:

```graphql
query GetBooks {
  favorite {
    title
    ... on Novel {
      protagonist
    }
    ... on Dictionary {
      language
    }
  }
}
```

genqlient will generate the following types (see [below](#-genqlient-generate-such-complicated-type-names) for more on the names):

```go
type GetBooksFavoriteBook interface {
  GetTitle() string
}
type GetBooksFavoriteNovel struct {
  Title string
  Protagonist string
}
type GetBooksFavoriteDictionary struct {
  Title string
  Language string
}
// (similarly for any other types that implement Book)
```

These can be used in the ordinary Go ways: to access shared fields, use the interface methods; to access type-specific fields, use a type switch:

```go
resp, err := GetBooks(...)
fmt.Println("Favorite book:", resp.Favorite.GetTitle())
if novel, ok := resp.Favorite.(*GetBooksFavoriteNovel); ok {
  fmt.Println("Protagonist:", novel.Protagonist)
}
```

The interface-type's GoDoc will include a list of its implementations, for your convenience.

If you only want to request shared fields of the interface (i.e. no fragments), this may seem like a lot of ceremony.  If you prefer, you can instead add `# @genqlient(struct: true)` to the field, and genqlient will just generate a struct, like it does for GraphQL object types.  For example, given:

```graphql
query GetBooks {
  # @genqlient(struct: true)
  favorite {
    title
  }
}
```

genqlient will generate just:

```go
type GetBooksFavoriteBook struct {
  Title string
}
```

Keep in mind that if you later want to add fragments to your selection, you won't be able to use `struct` anymore; when you remove it you may need to update your code to replace `.Title` with `.GetTitle()` and so on.


### … shared types between different parts of the query?

Suppose you have a query which requests several different fields each of the same GraphQL type, e.g. `User` (or `[User]`):

```graphql
query GetMonopolyPlayers {
  game {
    winner { id name }
    banker { id name }
    spectators { id name }
  }
}
```

This will produce a Go type like:
```go
type GetMonopolyPlayersGame struct {
  Winner     GetMonopolyPlayersGameWinnerUser
  Banker     GetMonopolyPlayersGameBankerUser
  Spectators []GetMonopolyPlayersGameSpectatorsUser
}

type GetMonopolyPlayersGameWinnerUser struct {
  Id   string
  Name string
}

// (others similarly)
```

But maybe you wanted to be able to pass all those users to a shared function (defined in your code), say `FormatUser(user ???) string`.  That's no good; you need to put three different types as the `???`.  genqlient has several ways to deal with this.

**Fragments:** One option -- the GraphQL Way, perhaps -- is to use fragments.  You'd write your query like:

```graphql
fragment MonopolyUser on User {
  id
  name
}

query GetMonopolyPlayers {
  game {
    winner { ...MonopolyUser }
    banker { ...MonopolyUser }
    spectators { ...MonopolyUser }
  }
}
```

genqlient will notice this, and generate a type corresponding to the fragment; `GetMonopolyPlayersGame` will look as before, but each of the field types will have a shared embed:

```go
type MonopolyUser struct {
  Id   string
  Name string
}

type GetMonopolyPlayersGameWinnerUser struct {
  MonopolyUser
}

// (others similarly)
```

Thus you can have `FormatUser` accept a `MonopolyUser`, and pass it `game.Winner.MonopolyUser`, `game.Spectators[i].MonopolyUser`, etc.  This is convenient if you may later want to add other fields to some of the queries, because you can still do

```graphql
fragment MonopolyUser on User {
  id
  name
}

query GetMonopolyPlayers {
  game {
    winner {
      winCount
      ...MonopolyUser
    }
    banker {
      bankerRating
      ...MonopolyUser
    }
    spectators { ...MonopolyUser }
  }
}
```

and you can even spread the fragment into interface types.  It also avoids having to list the fields several times.

**Fragments, flattened:** the field `Winner`, above, has type `GetMonopolyPlayersGameWinnerUser` which just wraps `MonopolyUser`.  If we don't want to add any other fields, that's unnecessary!  Instead, we could do
```
query GetMonopolyPlayers {
  game {
    # @genqlient(flatten: true)
    winner {
      ...MonopolyUser
    }
    # (etc.)
  }
}
```
and genqlient will skip the indirection and give the field `Winner` type `MonopolyUser` directly.  This is often much more convenient if you put all the fields in the fragment, like the first query did.  See the [options documentation](genqlient_directive.graphql) for more details.

**Type names:** Finally, if you always want exactly the same fields, you can use the simpler but more restrictive genqlient option `typename`:

```graphql
query GetMonopolyPlayers {
  game {
    # @genqlient(typename: "User")
    winner { id name }
    # @genqlient(typename: "User")
    banker { id name }
    # @genqlient(typename: "User")
    spectators { id name }
  }
}
```

This will tell genqlient to use the same types for each field:

```go
type GetMonopolyPlayersGame struct {
  Winner     User
  Banker     User
  Spectators []User
}

type User struct {
  Id   string
  Name string
}
```

In this case, genqlient will validate that each type given the name `User` has the exact same fields; see the [full documentation](genqlient_directive.graphql) for details.

**Bindings:** It's also possible to use the `bindings` option (see [`genqlient.yaml` documentation](genqlient.yaml)) for a similar purpose, but this is not recommended as it typically requires more work for less gain.

### … documentation on the output types?

For any GraphQL types or fields with documentation in the GraphQL schema, genqlient automatically includes that documentation in the generated code's GoDoc.  To add additional information to genqlient entrypoints, you can put comments in the GraphQL source:

```graphql
# This query gets the current user.
#
# If you also need to specify options on the query, you can put
# the @genqlient directive after the docuentation, like this:
#
# @genqlient(omitempty: true)
query GetUser { ... }
```

## Why does…

### … genqlient generate such complicated type-names?

The short answer is that GraphQL forces our hand.  For example, consider a query
```graphql
query GetFamilyNames {
  user {
    name
    children {
      name
    }
  }
}
```
which returns the following JSON:
```graphql
{
  "user": {
    "name": "Ellis Marsalis Jr.",
    "children": [
      {"name": "Branford Marsalis"},
      {"name": "Delfeayo Marsalis"},
      {"name": "Jason Marsalis"},
      {"name": "Wynton Marsalis"}
    ]
  }
}
```
We need two different `User` types to represent this: one with a `Children` field, and one without.  (And there may be more in other queries!)  Of course, we could name them `User1` and `User2`, but that's both less descriptive and less stable as the query changes (perhaps to add `parent`), so we call them `GetFamilyNamesUser` and `GetFamilyNamesUserChildrenUser`.

For the long answer, see [DESIGN.md](DESIGN.md#named-vs-unnamed-types).

If you find yourself needing to reference long generated names, you can always add type aliases for them, e.g.:
```go
type User = GetFamilyNamesUser
type ChildUser = GetFamilyNamesUserChildrenUser
```

Alternately, you can use the `typename` option: if you query
```graphql
query GetFamilyNames {
  # @genqlient(typename: "User")
  user {
    name
    # @genqlient(typename: "ChildUser")
    children {
      name
    }
  }
}
```
genqlient will instead generate types with the given names.  (You'll need to avoid conflicts; see the [full documentation](genqlient_directive.graphql) for details.)

### … my editor/IDE plugin not know about the code genqlient just generated?

If your tools are backed by [gopls](https://github.com/golang/tools/blob/master/gopls/README.md) (which is most of them), they simply don't know it was updated.  In most cases, keeping the generated file (typically `generated.go`) open in the background, and reloading it after each run of `genqlient`, will do the trick.
