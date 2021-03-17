# Design decisions

This file contains a log of miscellaneous design decisions in genql.

## Types

The main work of this library is in the return types it generates: these are the main difference from other libraries like [shurcooL/graphql](https://github.com/shurcooL/graphql/).  Choosing how to generate these types is thus one of the main design decisions.

Luckily, there is plenty of prior art for generating types for GraphQL schemas, including:
- Apollo's [codegen](https://github.com/apollographql/apollo-tooling#apollo-clientcodegen-output) tool generates Swift, Scala, Flow, and TypeScript types
- [GraphQL Code Generator](https://www.graphql-code-generator.com/) generates types for TypeScript, C#, Java, and Flow
- Khan Academy has an in-house tool used to generate Flow types for our mobile apps (Khan folks, it's at `react-native/tools/graphql-flow` in the mobile repo)
- In Go, [gqlgen](https://github.com/99designs/gqlgen) generates similar types for the server side of things
- In Go, [shurcooL/graphql](https://github.com/shurcooL/graphql/) doesn't generate types, but uses types in a similar fashion
But of course all of these have somewhat different concerns, so we need to make our own choices.

### Optionality and pointers

GraphQL has optional and non-optional types: `String` may be null, but `String!` is always set.  Go does not: there's just `string`, and it always has the zero value `0`, but cannot be `null`.  Some libraries allow the use of pointers for optionality -- `*string` is optional, `string` is required -- but this perhaps [goes against the intended Go style](https://github.com/golang/go/issues/38298#issuecomment-634837296).

We could refuse to use pointers, which makes it hard for clients that do want to tell `0` from `null` (we could allow a configurable default); or we could allow their use on an opt-in basis (which also requires a configuration knob); or we could use them whenever a type is optional (which in many schemas is quite often); or we could use another representation of optionality, like `MyField string; HasMyField bool` (or keep the non-optional field and add a method for presence, or whatever).

This is an important problem because many GraphQL APIs, including Khan Academy's, have a lot of optional fields.  Some are in practice required, or are omitted only if you don't have permissions!  So we don't want to add too much overhead for fields just because the schema considers them optional.

A related issue is whether large structs (or all structs) should use pointers.  This can be a performance benefit for large structs, as non-pointers would need to be copied on return.  We don't strictly need that except at the toplevel response type (where there is no notion of optionality anyway), but using it might encourage people to avoid passing pointers.

In other libraries:
- Libraries in many other languages, including JS, don't have this issue; and GraphQL Code Generator in C# doesn't really generate server types.
- gqlgen uses pointers for both optionality and all struct types; this has caused confusion for us.  For them the second problem is harder, as all struct types are return values.
- shurcooL/graphql allows but doesn't require pointers; in practice we tend to use them infrequently on an opt-in basis.

Here we can also look at non-GraphQL libraries:
- encoding/json, and the other stdlib encoding packages, allow pointers, but typically encourage other approaches like `omitempty` (as discussed in the above link).
- Google Cloud Datastore basically follows encoding/json; at Khan Academy we mostly don't use pointers
- protobuf uses pointers for all structs; proto2 uses pointers for all fields as well (although provides a non-pointer accessor function) whereas proto3 does not use pointers (and application is not supposed to be able to distinguish between zero and unset in any language).

In general I feel uncomfortable using pointers by default, because I think it's bad Go style.  The best option might be to allow them on an opt-in basis?  Then you could use them if you want to pass around intermediate values, or distinguish zero and null, or whatever your reason is.

### Named vs. unnamed types

Go has a strong distinction between named types (a.k.a. ["defined types"](https://golang.org/ref/spec#Type_definitions), for example, `type MyType struct { ... }`) and unnamed types (defined inline, as for example `struct { ... }`).  We have a choice in how much we want to use named vs. unnamed types.

Specifically, here are two ways we could generate the type for a simple query `query myQuery { user { id } }`:

```go
// unnamed types
type MyQueryResponse struct {
    User struct {
        ID string
    }
}

// named types
type MyQueryResponse struct {
    User User
}

type User struct {
    ID string
}
```

(In principle, we could go even further, and make MyQueryResponse a type alias or define it inline in the response type of the function `MyQuery`, but it's not clear this has much value.)

Each has its advantages.  Named types are the easiest to refer to -- if you want to write a function
```
func MakeMyQueryAndReturnUser(...) (???, error) {
    resp, err := MyQuery(...)
    return resp.User, err
}
```
it's much easier to type in the named types world -- `???` is just `User`, whereas with unnamed types it's `struct { ID string }` or some nonsense like that.  (Some type systems have a way to say "the type of T's field F", but Go's doesn't.)  This is especially relevant in tests, which may want to construct a value of type `MyQueryResponse`.  They are also necessary to some approaches to dealing with interfaces and fragments (see the next section).

But named types also cause a big problem, which is the names.  We can't actually necessarily just name the type above `User` -- there may be multiple `User` types in a single package, or even in a single query.  For example, the query `{ user { id children { id } } }` needs to look something like:

```go
type MyQueryResponse struct {
    User User1
}

type User1 struct {
    ID string
    Children []User2
}

type User2 struct {
    ID string
}
```

Moreover, these types names should be as stable as possible: changing one part of a query (or another query) shouldn't change the type-names.  And ideally, we might deduplicate: in a query like `{ self { id } myChildren { id } }` we'd be able to use a single type for both `self` and `myChildren`.  Although maybe, if you want that, you have to use a fragment.

In other tools:
- Apollo mostly uses named types (except alternatives of an interface/fragment are inline in TypeScript, but not in Flow), and in the above example names them, respectively, `MyQuery_user_User` and `MyQuery_user_User_children_User`, or maybe in some versions `MyQuery_user` and `MyQuery_user_children` in Flow/TypeScript.  In Java and Scala in some cases they use nested types, e.g. `MyQuery.User.Children`.
- GraphQL Code Generator generates mostly unnamed types in Flow/TypeScript, with named types for named fragments only (except really it generates server-style types and then generates a mess of unnamed `$Pick`s from those); and basically just generates server-style types for other languages
- Khan's mobile autogen uses entirely unnamed types.
- gqlgen doesn't have this problem, because on the server each GraphQL type maps to a unique Go type; and to the extent that different queries need different fields this is handled at the level of which fields are serialized or which resolvers are called.
- shurcooL/graphql allows either way.

In general, it seems like even in languages with better ergonomics for unnamed types, Apollo's approach is somewhat reasonable.  And unnamed types get will get really hairy for large queries in Go.  We'll need to decide the naming scheme, but we can copy Apollo's if needed.  On some level, even if the naming scheme is bad, it won't be as bad as unnamed types!  But it is somewhat hard to change later (at least without some sort of backcompat), as it breaks existing generated code.

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

(This requires that these types, at least, be named, in the sense of the previous section.)  In this approach, the two objects above would look like

```go
Response{A: T{B: "...", C: "...", D: "..."}}
Response{A: U{B: "..."}}
```

Another natural option, which looks more like the way `shurcooL/graphql` does things, is to generate a type for each fragment, and only fill in the relevant ones:
```go
type Response struct {
    A struct {
        B string
        F struct { // optional
            C, D string
        }
    }
}
```

(This works with named or unnamed types as discussed in the previous section.)  In this approach, the two objects above would look like

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

- it's the most natural translation of how GraphQL does things, and provides the clearest guarantees about what is set when, what is mutually exclusive, etc.
- you always know what type you got back
- you always know which fields are there -- you don't have to encode at the application level an assumption that if fragment A was defined, fragment B also will be, because all types that match A also match B

Pros of the second approach:

- the types are simpler, and allow the option of unnamed types
- if you query an interface, but don't care about the types, just the shared fields, you don't have to worry about anything special
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

In other libraries:
- Apollo does basically option 1, except with TypeScript/Flow's better support for sum types.
- GraphQL Code Generator does basically option 1 (except with sum types instead of interfaces), except with unnamed types.  It definitely generates some ugly types, even with TypeScript/Flow's better support for sum types!
- Khan's mobile autogen basically does option 1 (again with unnamed types, and sum types).
- gqlgen doesn't have this problem; fragments and interfaces don't need special handling by user-land server code.
- shurcooL/graphql uses option 2.
- protobuf has a similar problem, and uses basically option 1 in Go.

In general, it seems like the GraphQL Way, and perhaps also the Go Way is definitely Option 1; I've always found the way shurcooL/graphql handles this to be questionable, and it requires a custom JSON decoder (although we may end up needing that either way, because Go doesn't have a great way to decode sum types).

## Configuration and runtime

### General configuration

We will need some configuration for the library.  Global configuration we can easily put in a file, but per-query configuration makes the most sense inline.  We will eventually want to configure things like:
- optionality, perhaps (see above)
- collapsing (e.g. a query `{ a { b { c } } }` could, if configured, just return the `c`, so you don't have to unpack)

One option is to put configuration in GraphQL directives.  The problem is that the server won't understand them!  So we'd have to remove them from the query; this means we're not sending the exact query we got to the server, which could mess with safelisting, hashing, etc.

Instead, we will likely have to do that configuration in comments, which we already extract to use in docstrings.  We'll have to figure out how to support both.

Or, we can figure out how to avoid configuration entirely.  This seems short-sighted but may be possible if we can solve for optionality another way and decide collapsing is a non-issue.

### Query function signatures (context/client)

Many users will want to give us a context.  Some may want to customize their HTTP client in arbitrary ways.  Everyone needs a place to thread in the URL.  A common way to handle the latter issues is with a client object.

Given that, here are some sample signatures for a function that accepts a single ID variable:

```go
// None of the above
func GetUser(id string) (*GetUserResponse, error)

// Uses a standard context
func GetUser(ctx context.Context, id string) (*GetUserResponse, error)

// Uses a custom context (used at Khan; this may become more common post-generics)
func GetUser(ctx mypkg.Context, id string) (*GetUserResponse, error)

// Uses a client object
func GetUser(client graphql.Client, id string) (*GetUserResponse, error)

// Uses both
func GetUser(ctx context.Context, client graphql.Client, id string) (*GetUserResponse, error)
```

Additionally, users may want to get the client from the context, using a custom method like Khan's KAContext or just ordinary `context.Value`.

This can all be configurable globally -- say you can decide whether to use context and client, and optionally provide the type of your context and/or a function that gets client from it, or something.  We'll want to pick a good default before we have external users, so as not to break them, but it's easy enough to change the Khan-specific parts via codemod later.
