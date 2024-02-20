# Frequently Asked Questions

This document describes common questions about genqlient, and provides an index to how to represent common query structures.  For a full list of configuration options, see [genqlient.yaml](genqlient.yaml) and [genqlient_directive.graphql](genqlient_directive.graphql).

## Configuring genqlient

### How do I set up genqlient to use an API that requires authentication?

Customize the `http.Client` of your [`graphql.Client`](client_config.md#authentication-and-other-headers).

### How do I make requests against a mock server, for tests?

Inject a test HTTP response or server [into the `graphql.Client`](client_config.md#testing).

### Does genqlient support custom scalars?

Tell genqlient how to handle your custom scalars with the [`bindings` option](schema.md#custom-scalars).

### Can I use introspection to fetch my client schema?

Yes, but you'll need to use a separate tool ([example](schema.md#fetching-your-schema)).

## Why?

### Why use genqlient?

See the [README.md](../README.md#why-another-graphql-client).

### Why does genqlient generate such complicated type-names?

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

This applies even in cases where the types are exactly the same, so that the type names will be stable as the query changes. For example, in the following query there are three different "User" types:

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

This may seem unnecessary, but imagine if we generated a single type `GetMonopolyPlayersGameUser`, and then later you changed the query to do `spectators { id name favoritePlayer }`; we'd now have to change all three type names, potentially forcing you to update all the code that uses the query result.

For more on customizing the type names -- including combining several types into one -- see the [operations documentation](operations.md#type-names).

For the long answer, see the [design note](design.md#named-vs-unnamed-types).

## Known issues

### My editor/IDE plugin doesn't know about the code genqlient just generated

If your tools are backed by [gopls](https://github.com/golang/tools/blob/master/gopls/README.md) (which is most of them), they simply don't know it was updated.  In most cases, keeping the generated file (typically `generated.go`) open in the background, and reloading it after each run of `genqlient`, will do the trick.

### genqlient fails after `go mod tidy`

If genqlient fails with an error `missing go.sum entry for module providing package`, this is typically because `go mod tidy` removed its dependencies  because they weren't imported by your Go module.  You can read more about this in golang/go#45552; see in particular [this comment](https://github.com/golang/go/issues/45552#issuecomment-819545037).  In short, if you want to be able to `go run` on newer Go you'll need to have a (blank) import of genqlient's entrypoint in a special `tools.go` file somewhere in your module so `go mod tidy` doesn't prune it:

```go
//go:build tools
// +build tools

package client

import _ "github.com/Khan/genqlient"
```

### I'm getting confusing errors from `@genqlient` directives

Currently, `@genqlient` directives apply to all relevant nodes on the following line, see [#151](https://github.com/Khan/genqlient/issues/151) or the [`@genqlient` documentation](genqlient_directive.graphql). If in doubt, spread things out onto more lines and they'll probably work!

Common examples of this error:
- `for is only applicable to operations and arguments`
- `omitempty may only be used on optional arguments`

### My issue is fixed in `main` but not in the latest release

genqlient does not publish a release for every bugfix; read more about our [versioning strategy](versioning.md) or use `go get -u github.com/Khan/genqlient@main` to install from latest `main`.
