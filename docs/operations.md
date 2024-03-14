# Writing your GraphQL operations for genqlient

While simple query structures map naturally from GraphQL to Go, more complex queries require different handling. This document describes how genqlient maps various GraphQL structures to Go, and the best ways to structure your queries and configure genqlient to handle them. For a complete list of options, see the [`genqlient.yaml`](genqlient.yaml) and [`@genqlient`](genqlient_directive.graphql) references.

## Nullable fields

There are several ways to handle nullable fields in genqlient: using [zero values](#zero-values), [pointers](#pointers), or [generics](#generics). In some cases you'll also need [`omitempty`](#omitempty).

### Zero values

One way is to use the Go idiom, where null gets mapped to the zero value; this is the default in genqlient.  So if you have a GraphQL field of type `String`, and you do:

```graphql
query MyQuery(arg: String) {
  myString
}
```

then genqlient will generate a Go field `MyString string`, and set it to the empty string if the server returns null.  This works even for structs: if an object type in GraphQL is null, genqlient will set the corresponding struct to its zero value.  It can be helpful to request `id` in such cases, since that’s a field that should always be set, or `__typename` which is guaranteed to be set, so you can use its presence to decide whether to look at the other fields.

### omitempty

For input fields, you often want to tell genqlient to send null to the server if the argument is set to the zero value, similar to the JSON `omitempty` tag.  In this case, you can do:

```graphql
query MyQuery(
  # @genqlient(omitempty: true)
  arg: String,
) {
  myString
}
```

You can also put the `# @genqlient(omitempty: true)` on the first line, which will apply it to all arguments in the query, or `# @genqlient(for: "MyInput.myField", omitempty: true)` on the first line to apply it to a particular field of a particular input type used by the query (for which there would otherwise be no place to put the directive, as the field never appears explicitly in the query, but only in the schema).

Note that omitempty doesn't apply to structs, just like `encoding/json`. For structs that may be entirely unset, you can use `# @genqlient(omitempty: true, pointer: true)`, since `nil` pointers are omitted.

### Generics

If you need to distinguish null from the empty string (or generally from the Go zero value of your type), you can tell genqlient to use a generic type for optional fields/arguments, similar to Rust's `Option<T>`.

You can configure this by defining the type to use in your code (or using one from a library), for example:
```go
type Option[T any] struct {
    Value T
    HasValue bool
}

// MarshalJSON, UnmarshalJSON, constructors, etc.
```

Then tell genqlient to use it like so:
```yaml
optional: generic
optional_generic_type: github.com/path/to/your/package.Option
```

This will generate a Go field `MyString Option[string]`, which you can handle as desired.

### Pointers

Similar to generics, you can tell genqlient to use a pointer for a field or argument:
```graphql
query MyQuery(
  # @genqlient(pointer: true)
  arg: String,
) {
  # @genqlient(pointer: true)
  myString
}
```

This will generate a Go field `MyString *string`, and set it to `nil` if the server returns null (and in reverse for arguments).  Such fields can be harder to work with in Go, but allow a clear distinction between null and the Go zero value. You can put `optional: pointer` to apply this to all optional fields, or put the directive on the first line to apply it to everything in the query, although both often get cumbersome. To apply it to a specific input-type field, use `for`:

As an example of using all these options together:
```graphql
# @genqlient(omitempty: true)
# @genqlient(for: "MyInputType.id", omitempty: false, pointer: true)
# @genqlient(for: "MyInputType.name", omitempty: false, pointer: true)
query MyQuery(
  arg1: MyInputType!,
  # @genqlient(pointer: true)
  arg2: String!,
  # @genqlient(omitempty: false)
  arg3: String!,
) {
  myString(arg1: $arg1, arg2: $arg2, arg3: $arg3)
}
```
This will generate:
```go
func MyQuery(
  ctx context.Context,
  client graphql.Client,
  arg1 MyInputType,
  arg2 *string, // omitempty
  arg3 string,
) (*MyQueryResponse, error)

type MyInputType struct {
  Id    *string `json:"id"`
  Name  *string `json:"name"`
  Title string  `json:"title,omitempty"`
  Age   int     `json:"age,omitempty"`
}
```

## GraphQL Interfaces

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

## Sharing types

By default, genqlient generates a different type for each part of each query, [even those which are structurally the same](faq.md#why-does-genqlient-generate-such-complicated-type-names-). Sometimes, however, you want to have some common code that can accept data from several queries or parts of queries.

For example, suppose you have a query which requests several different fields each of the same GraphQL type, e.g. `User` (or `[User]`):

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

// (other structs identical)
```

But maybe you wanted to be able to pass all those users to a shared function (defined in your code), say `FormatUser(user ???) string`.  That's no good; you need to put three different types as the `???`.

genqlient has several ways to deal with this. The two best methods for most uses are [fragments](#fragments), useful for reuse that exactly matches the query; and [Go interfaces](#go-interfaces), useful for more flexible access to types with common fields. For some use cases, the [`typename`](#shared-type-names) and [`bindings`](#bindings) options can be useful.

### Fragments

One option -- the GraphQL Way, perhaps -- is to use fragments.  You'd write your query like:

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

### Flattening fragments
The Go field for `winner`, in the first query above, has type `GetMonopolyPlayersGameWinnerUser` which just wraps `MonopolyUser`.  If we don't want to add any other fields, that's unnecessary!  Instead, we could do
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
and genqlient will skip the indirection and give the field `Winner` type `MonopolyUser` directly.  This is often much more convenient if you put all the fields in the fragment, like the first query did.

### Go interfaces

For each struct field it generates, genqlient also generates an interface method.  If you want to share code between two types which to GraphQL are unrelated, you can define an interface containing that getter method, and genqlient's struct types will implement it.  (Depending on your exact query, you may need to do a type-assertion from a genqlient-generated interface to yours.)  For example, in the above query you could simply do:
```go
type MonopolyUser interface {
    GetId() string
    GetName() string
}

func FormatUser(user MonopolyUser) { ... }

FormatUser(resp.Game.Winner)
```

In general in such cases it's better to change the GraphQL schema to show how the types are related, and use one of the other mechanisms, but this option is useful for schemas where you can't do that, or in the meantime.

### Shared type names

Finally, if you always want exactly the same fields on exactly the same types, and don't want to deal with interfaces at all, you can assign the same type name to multiple fields

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

See [below](#type-names) for more on this option.

### Bindings

It's also possible to use the `bindings` option (see [`genqlient.yaml` documentation](genqlient.yaml)) for a similar purpose, but this is not recommended as it typically requires more work for less gain.

## Names

### Operation names

genqlient will use the exact name of your query as the generated function name. For example, if your query looks like `query myQuery { ... }`, then genqlient will generate `func myQuery(...) (*myQueryResponse, error)`. This means your queries should follow the usual Go conventions, especially starting with an uppercase letter if the query should be exported.

### Field names

By default, genqlient chooses field names based on the schema's field names. To customize the name, genqlient supports GraphQL field-aliases.  For example, if you do
```graphql
query MyQuery {
  myGreatName: myString
}
```
and genqlient will generate a Go field `MyGreatName string`.  Note that the alias will always be uppercased, to ensure the field is visible to the Go JSON library.

### Type names

genqlient generates quite verbose type names in many cases. (In short, this is because the same `User` GraphQL type must map to different Go types depending which fields are requested; see the FAQ for [more](faq.md#why-does-genqlient-generate-such-complicated-type-names-).

For example, in the following query there are two different user structs.
```graphql
query GetFamilyNames {
  user { # GetFamilyNamesUser
    name
    children { # GetFamilyNamesUserChildrenUser
      name
    }
  }
}
```

In many cases, you won't need to refer to these directly (only the field names, which are short). But when you do, you can add type aliases for them in your code:
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
genqlient will instead generate types with the given names. You can even [map multiple identical types to one](#shared-type-names).

This approach can be quite convenient, but you'll need to take care to avoid conflicts: each name must be unique, unless the fields are exactly the same (see the [full documentation](genqlient_directive.graphql) for details).

## Documentation

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

