# Design decisions

This file contains a log of miscellaneous design decisions in genqlient.  They aren't all necessarily up to date with the exact implementation details, but are preserved here as context for why genqlient does X thing Y way.

## Types

The main work of this library is in the return types it generates: these are the main difference from other libraries like [shurcooL/graphql](https://github.com/shurcooL/graphql/).  Choosing how to generate these types is thus one of the main design decisions.

Luckily, there is plenty of prior art for generating types for GraphQL schemas, including:
- Apollo's [codegen](https://github.com/apollographql/apollo-tooling#apollo-clientcodegen-output) tool generates Swift, Scala, Flow, and TypeScript types
- [GraphQL Code Generator](https://www.graphql-code-generator.com/) generates types for TypeScript, C#, Java, and Flow
- infiotinc Academy has an in-house tool used to generate Flow types for our mobile apps (infiotinc folks, it's at `react-native/tools/graphql-flow` in the mobile repo)
- In Go, [gqlgen](https://github.com/99designs/gqlgen) generates similar types for the server side of things
- In Go, [shurcooL/graphql](https://github.com/shurcooL/graphql/) doesn't generate types, but uses types in a similar fashion

But of course all of these have somewhat different concerns, so we need to make our own choices.

### Optionality and pointers

GraphQL has optional and non-optional types: `String` may be null, but `String!` is always set.  Go does not: there's just `string`, and it always has the zero value `""`, but cannot be `nil`.  Some libraries allow the use of pointers for optionality -- `*string` is optional, `string` is required -- but this perhaps [goes against the intended Go style](https://github.com/golang/go/issues/38298#issuecomment-634837296).

We could refuse to use pointers, which makes it hard for clients that do want to tell `""` from `nil` (we could allow a configurable default); or we could allow their use on an opt-in basis (which also requires a configuration knob); or we could use them whenever a type is optional (which in many schemas is quite often); or we could use another representation of optionality, like `MyField string; HasMyField bool` (or keep the non-optional field and add a method for presence, or whatever).

This is an important problem because many GraphQL APIs, including infiotinc Academy's, have a lot of optional fields.  Some are in practice required, or are omitted only if you don't have permissions!  So we don't want to add too much overhead for fields just because the schema considers them optional.

A related issue is whether large structs (or all structs) should use pointers.  This can be a performance benefit for large structs, as non-pointers would need to be copied on return.  We don't strictly need that except at the toplevel response type (where there is no notion of optionality anyway), but using it might encourage people to avoid passing pointers.

In other libraries:
- Libraries in many other languages, including JS, don't have this issue; and GraphQL Code Generator in C# doesn't really generate server types.
- gqlgen uses pointers for both optionality and all struct types; this has caused confusion for us.  For them the second problem is harder, as all struct types are return values.
- shurcooL/graphql allows but doesn't require pointers; in practice we tend to use them infrequently on an opt-in basis.

Here we can also look at non-GraphQL libraries:
- encoding/json, and the other stdlib encoding packages, allow pointers, but typically encourage other approaches like `omitempty` (as discussed in the above link).
- Google Cloud Datastore basically follows encoding/json; at infiotinc Academy we mostly don't use pointers
- protobuf uses pointers for all structs; proto2 uses pointers for all fields as well (although provides a non-pointer accessor function) whereas proto3 does not use pointers (and application is not supposed to be able to distinguish between zero and unset in any language).

**Decision:** I do not want to use pointers by default, because I really think it's just terrible Go style.  I think we can actually get away with just not supporting this at all for v0, and then add config options for pointers (which you can use for distinguishing zero and null, or to pass around intermediate values) and possibly also for presence fields, or anything else you want.

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

Moreover, these types names should be as stable as possible: changing one part of a query (or another query) shouldn't change the type-names.  And ideally, we might deduplicate: in a query like `{ self { id } myChildren { id } }` we'd be able to use a single type for both `self` and `myChildren`.  Although maybe, if you want that, you have to use a fragment; it's clearly in conflict with having stable names.  Or we can actually make this configurable!

In other tools:
- Apollo mostly uses named types (except alternatives of an interface/fragment are inline in TypeScript, but not in Flow), and in the above example names them, respectively, `MyQuery_user_User` and `MyQuery_user_User_children_User`, or maybe in some versions `MyQuery_user` and `MyQuery_user_children` in Flow/TypeScript.  In Java and Scala in some cases they use nested types, e.g. `MyQuery.User.Children`.
- GraphQL Code Generator generates mostly unnamed types in Flow/TypeScript, with named types for named fragments only (except really it generates server-style types and then generates a mess of unnamed `$Pick`s from those); and basically just generates server-style types for other languages
- infiotinc's mobile autogen uses entirely unnamed types.
- gqlgen doesn't have this problem, because on the server each GraphQL type maps to a unique Go type; and to the extent that different queries need different fields this is handled at the level of which fields are serialized or which resolvers are called.
- shurcooL/graphql allows either way.

A side advantage of an approach that prefixes the query name to all the type names, for us, is that it means that the caller can decide in a very simple way whether to make the name exported, and avoids conflicts between queries.

**Decision:** In general, it seems like even in languages with better ergonomics for unnamed types, Apollo's approach is somewhat reasonable.  And unnamed types will get really hairy for large queries in Go.  On some level, even if the naming scheme is bad, it won't be as bad as unnamed types -- if you don't need to refer to the intermediate type, you don't care, and if you do, it's better than a giant inline struct.  (But it's hard to change later without a flag as existing code may depend on those types.)

We'll do something similar to Apollo's naming scheme.  Specifically:
- The toplevel name will be `MyQueryResponse`, using the query-name.
- Further names will be `MyQueryFieldTypeFieldType`.  We will not attempt to be super smart about avoiding conflicts.
- Fragments will have some naming scheme TBD but starting at the fragment.
- Input objects will have a name starting at the type, since they always have the same fields, and often have naming schemes like "MyFieldInput" already.

See `generate/names.go` for the precise algorithm.

All of this may be configurable later.

### How to represent interfaces

Consider the following query (suppose that `a` returns interface type `I`, which may be implemented by either `T` or `U`):

```graphql
query { a { __typename b ...F } }
fragment F on T { c d }
```

Depending on whether the concrete type returned from `a` is `T`, we can get one of two result structures back:

```json
{"a": {"__typename": "T", "b": "...", "c": "..." , "d": "..."}}
{"a": {"__typename": "U", "b": "..."}}
```

The question is: how do we translate that to Go types?

**Go interfaces:** one natural option is to generate a Go type for every concrete GraphQL type the object might have, and simply inline or embed all the fragments.  So here we would have
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

We can also define a getter-method `GetB() string` on `T` and `U`, and include it in `I`, so that if you only need `B` you don't need to type-switch.  Or, if that's not enough, we can also define a common embedded struct corresponding to the interface, so you can extract and pass that around if you want:

```go
type IEmbed struct { B string }
type T struct { IEmbed; C, D string }
type U struct { IEmbed }
type I interface { isI(); GetIEmbed() IEmbed }
```

Note that this option gives a few different ways to represent fragments specifically, discussed in the next section.

**Fragment fields:** another natural option, which looks more like the way `shurcooL/graphql` does things, is to generate a type for each fragment, and only fill in the relevant ones:
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


**Pros of Go interfaces:**

- it's the most natural translation of how GraphQL does things, and provides the clearest guarantees about what is set when, what is mutually exclusive, etc.
- you always know what type you got back
- you always know which fields are there -- you don't have to encode at the application level an assumption that if fragment A was defined, fragment B also will be, because all types that match A also match B

**Pros of fragment fields:**

- the types are simpler, and allow the option of unnamed types
- if you query an interface, but don't care about the types, just the shared fields, you don't even have to think about any of this stuff
- for callers accessing shared fields (of interfaces or of fragments spread into several places) we avoid having to make them do a type switch or use getter methods

Note that both approaches require that we add `__typename` to every selection set which has fragments (unless the types match exactly).  This seems fine since Apollo client also does so for all selection sets.  We also need to define `UnmarshalJSON` on every type with fragment-spreads; in the former case Go doesn't know how to unmarshal into an interface type, while in the latter the Go type structure is too different from that of the JSON.  (Note that `shurcooL/graphql` actually has its own JSON-decoder to solve this problem.)

**Flatten everything:** a third non-approach is to simplify define all the fields on the same struct, with some optional:

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

```graphql
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

**In other libraries:**
- Apollo does basically the interface approach, except with TypeScript/Flow's better support for sum types.
- GraphQL Code Generator does basically the interface approach (except with sum types instead of interfaces), except with unnamed types.  It definitely generates some ugly types, even with TypeScript/Flow's better support for sum types!
- infiotinc's mobile autogen basically does the interface approach (again with unnamed types, and sum types).
- gqlgen doesn't have this problem; on the server, fragments and interfaces are handled entirely in the framework and need not even be visible in user-land.
- shurcooL/graphql uses fragment fields.
- protobuf has a similar problem, and uses basically the interface approach in Go (even though in other languages it uses something more like flattening everything).

**Decision:** In general, it seems like the GraphQL Way, and perhaps also the Go Way is to use Go interfaces; I've always found the way shurcooL/graphql handles this to be a bit strange.  So I think it has to be at least the default.  In principle we could allow both though, since fragment fields are legitimately convenient for some cases, especially if you are querying shared fields of interfaces or using fragments as a code-sharing mechanism, since using Go interfaces handles those only through getter methods (although see below).

### How to support fragments

The previous section leaves some parts of how we'll handle fragments ambiguous.  Even within the way it lays things out, we generally have two options for how to represent a fragment.  Consider a query

```graphql
query MyQuery { a { b ...F } }
fragment F on T { c d }
```

We'll have some struct (potentially several structs, one per implementation) representing the type of the value of `a`.  We can handle the fragment one of two ways: we can either flatten it into that type, such that the code is equivalent to writing `query Q { a { b c d } }` (except that `c` and `d` might be included for only some implementations, if `a` returns an interface), or we can represent it as a separate type `F` and embed it in the relevant structs.  (Or it could be a named field of said structs, but this seems to have little benefit, and it's nice to be able to access fields without knowing what fragment they're on.)  When the spread itself is abstract, there are a few sub-cases of the embed option.

To get more concrete, there are [four cases](https://spec.graphql.org/June2018/#sec-Fragment-spread-is-possible) depending on the types in question.  (In all cases below, the methods, and other potential implementations of the interfaces, are elided.  Note also that I'm not sure I got all the type-names right exactly as genqlient would, but they should match approximately.)

**Object spread in object scope:** The simplest spread is when we have an object-typed fragment spread into an object-typed selection.  (The two object types must be the same.)  This is typically used as a code-sharing mechanism.

```graphql
type Query { a: A }
type A { b: String, c: String, d: String }

query MyQuery { a { b ...F } }
fragment F on A { c d }
```

```go
// flattened:
type MyQueryA struct { B, C, D string }

// embedded
type MyQueryA struct { B string; F }
type F struct { C, D string }
```

**Abstract spread in object scope:** We can also spread an interface-typed fragment into an object-typed selection, again as a code-sharing mechanism.  (The object must implement the interface.)

```graphql
type Query { a: A }
type A implements I { b: String, c: String, d: String }
interface I { c: String, d: String }

query MyQuery { a { b ...F } }
fragment F on I { c d }
```

```go
// flattened:
type MyQueryA struct { B, C, D string }

// embedded:
type MyQueryA struct { B string; FA }
type F interface { isF(); GetC() string; GetD() string } // for code-sharing purposes
type FA struct { C, D string } // implements F
```

**Object spread in abstract scope:** This is the most common spread, perhaps, where you spread an object-typed fragment into an interface-typed selection in order to request some fields defined on a particular implementation of the interface.  (Again the object must implement the interface.)

```graphql
type Query { a: I }
type A implements I { b: String, c: String, d: String }
type T implements I { b: String, u: String, v: String }
interface I { b: String }

query MyQuery { a { b ...F ...G } }
fragment F on A { c d }
fragment G on A { u v }
```

```go
// flattened:
type MyQueryAI interface { isMyQueryAI(); GetB() string }
type MyQueryAIA struct { B, C, D string } // implements MyQueryAI
type MyQueryAIT struct { B, U, V string } // implements MyQueryAI

// embedded:
type MyQueryAI interface { isMyQueryAI(); GetB() string }
type MyQueryAIA struct { B string; F } // implements MyQueryAI
type MyQueryAIT struct { B string; G } // implements MyQueryAI
type F struct { C, D string }
type G struct { U, V string }
```

**Abstract spread in abstract scope:** This is a sort of combination of the last two, where you spread one interface's fragment into another interface, and can be used for code-sharing and/or to conditionally request fields.  Perhaps surprisingly, this is legal any time the two interfaces share an implementation, and neither need implement the other; this means there are arguably four cases of spreading a fragment of type `I` into a scope of type `J`: `I = J` (similar to object-in-object), `I implements J` (similar to abstract-in-object), `J implements I` (similar to object-in-abstract), and none of the above (which is quite rare).

```graphql
type Query { a: I }
type A implements I & J { b: String, c: String, d: String }
type T implements I { b: String }
type U implements J { c: String, d: String, v: String }
interface I { b: String }
interface J { c: String, d: String }

query MyQuery { a { b ...F } }
fragment F on J { c d }
```

```go
// flattened:
type MyQueryAI interface { isMyQueryAI(); GetB() string }
type MyQueryAIA struct { B, C, D string } // implements MyQueryAI (and MyQueryAJ if generated)
type MyQueryAIT struct { B }              // implements MyQueryAI

// embedded:
type MyQueryAI interface { isMyQueryAI(); GetB() string }
type MyQueryAIA struct { B string; FA } // implements MyQueryAI
type MyQueryAIT struct { B string } // implements MyQueryAI
type F interface { isF(); GetC() string; GetD() string }
type FA struct { C, D string } // implements F
type FU struct { C, D string } // implements F (never used; might be omitted)
// if I == J or I implements J, perhaps additionally:
type MyQueryAI interface { isMyQueryAI(); GetB() string; F }
```

Note in this case a third non-approach is

```go
// does not work:
type MyQueryAI interface { isMyQueryAI(); GetB() string }
type MyQueryAIA struct { B string; F } // implements MyQueryAI
type F struct { C, D string }
```

This doesn't work because the fragment F might itself embed other fragments of object type.

**Inline and named fragments:** Another complication is that in each of these cases, the fragment might be inline (`... on T { ... }`) or defined (`...F` where `fragment F on T { ... }`).  The latter is useful for sharing code, whereas the former may be more preferable when we just want request fields from a particular type.  Note that inline fragments have no natural name of their own; in the embedding approach we'd need to generate one.  (This is extra awkward because there's no prohibition on having several inline fragments of the same type in the same selection, e.g. `{ ... on T { id }, ... on T { name } }`, so even adding the type-name doesn't produce a unique name.)

**Pros of flattening:**

- Makes for the simplest resulting types, by far; the fragment adds no further complexity to the types.
- Especially a simplification when embedding abstract-typed fragments, since you don't have to go through an extra layer of interface when you already have a concrete type.
- More efficient when multiple fragments spread into the same selection contain the same field: we need only store it once whereas embedding must copy it once for each fragment.  Also easier to use in the same case, since if you have both `val.FragmentOne.MyField` and `val.FragmentTwo.MyField`, you can't access either via `val.MyField`.  (Empirically in the infiotinc Academy codebase this appears to be quite rare.)
- If you need to manually construct values of genqlient-generated types, flattening will be a lot easier, but I don't really recommend doing that.

**Pros of embedding:**

- Results in cleaner type-names, since each (named) fragment can act as its own "root" for naming purposes.  (In principle we could do this even when flattening, although it might read somewhat awkwardly.)
- Much more usable for deduplication; if you spread the fragment in several places in your query you can write a function which accepts an `F`, and pass it any of the data.  (Again we might be able to get this benefit, awkwardly, when flattening by generating an interface corresponding to each fragment, assuming we are also rooting the type-names at the fragment.)
- It's easier to tell what fields go where; the casework for abstract-in-abstract embeds gets somewhat complex.
- Arguably the most philosophically faithful representation of the GraphQL types in Go.

Note in principle we could apply some of those benefits 

**Decision:** There are pros and cons both ways here; in general it seems embedding is the most natural where your goal is deduplication, whereas flattening is best for inline fragments; for named fragments used only once there's maybe a slight benefit to flattening but it's not a big difference either way.  If we have to go with one or the other, probably flattening is better.  But I think the best thing, unless it turns out to be too much work to implement, is probably just to flatten inline fragments and embed named ones.  (In principle we could also have a flag flatten named fragments, if we find a need.)

## Configuration and runtime

### General configuration

We will need some configuration for the library.  Global configuration we can easily put in a file, but per-query configuration makes the most sense inline.  We will eventually want to configure things like:
- optionality, perhaps (see above)
- collapsing (e.g. a query `{ a { b { c } } }` could, if configured, just return the `c`, so you don't have to unpack)

One option is to put configuration in GraphQL directives.  The problem is that the server won't understand them!  So we'd have to remove them from the query; this means we're not sending the exact query we got to the server, which could mess with safelisting, hashing, etc.

Instead, we will likely have to do that configuration in comments, which we already extract to use in docstrings.  We'll have to figure out how to support both.

Or, we can figure out how to avoid configuration entirely.  This seems short-sighted but may be possible if we can solve for optionality another way and decide collapsing is a non-issue.

**Decision:** We'll configure with comments on the field of the form `# @genqlient(...)`, syntax TBD but similar to a GraphQL directive.

### Query function signatures (context/client)

Many users will want to give us a context.  Some may want to customize their HTTP client in arbitrary ways.  Everyone needs a place to thread in the URL.  A common way to handle the latter issues is with a client object.

Given that, here are some sample signatures for a function that accepts a single ID variable:

```go
// None of the above
func GetUser(id string) (*GetUserResponse, error)

// Uses a standard context
func GetUser(ctx context.Context, id string) (*GetUserResponse, error)

// Uses a custom context (used at infiotinc; this may become more common post-generics)
func GetUser(ctx mypkg.Context, id string) (*GetUserResponse, error)

// Uses a client object
func GetUser(client graphql.Client, id string) (*GetUserResponse, error)

// Uses both
func GetUser(ctx context.Context, client graphql.Client, id string) (*GetUserResponse, error)
```

Additionally, users may want to get the client from the context, using a custom method like infiotinc's KAContext or just ordinary `context.Value`.

This can all be configurable globally -- say you can decide whether to use context and client, and optionally provide the type of your context and/or a function that gets client from it, or something.  We'll want to pick a good default before we have external users, so as not to break them, but it's easy enough to change the infiotinc-specific parts via codemod later.

**Decision:** It seems easy enough to allow all of this to be configured: you can specify no context, a specific context type, or the default of context.Context; and then if you want you can specify a way to get the client from context or a global.  We'll need both hooks at infiotinc, and it's not much harder to add them in a generalizable way.

### Query extraction (for safelisting)

One thing we want to be able to do is to make it clear exactly what query-document (down to comments and whitespace) we will be sending in the query for the purposes of safelisting and querying based on hash.  In the case where you have one query per file, that's easy, just use the whole file.  But you may want to share fragments between queries, in which case this is trouble: you either need a way to include fragments from another file (and a defined concatenation order), or you need to have several queries per file, and either have genqlient extract the right parts (in a defined/reproducible way), or have it send up the full file and the operation name to use (in which case we should still encourage you to not do that unless you're hashing, so that you aren't sending up too much data).  We could also allow configuration between the last two options (so if you don't care about hashing/safelisting you can auto-extract).  A variant is to have genqlient write out a list of what it thinks the final queries will be (it has them to include in the generated code), in which case you don't have to care how smart that is.

We can decide this later once we support fragments and safelisting/hashing; it's fine if that's not available at first.
