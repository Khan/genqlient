# Configuring genqlient to use your GraphQL schema

This document describes common configuration options to get genqlient to work well with your GraphQL schema. For a complete list of options, see the [`genqlient.yaml` reference](genqlient.yaml).

## Fetching your schema

At present, genqlient expects your schema to exist on-disk. To fetch the schema from the server using introspection, you can use a tool such as [gqlfetch] and then let `genqlient` continue from there. Similarly, for [federated] servers you might fetch the supergraph (federated) schema from a registry, or construct it locally from the subgraph schemas.

[gqlfetch]: https://github.com/suessflorian/gqlfetch
[federated]: https://www.apollographql.com/docs/federation/

If desired, you can wrap this process up in a tool that you call via `go generate`, for example:

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Khan/genqlient/generate"
	"github.com/suessflorian/gqlfetch"
)

func main() {
	schema, err := gqlfetch.BuildClientSchema(context.Background(), "http://localhost:8080/query")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err = os.WriteFile("schema.graphql", []byte(schema), 0644); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	generate.Main()
}
```

This can now be invoked upon `go generate` via `//go:generate yourpkg/generate`.

## Scalars

GraphQL [defines][spec#scalar] five standard scalar types, which genqlient automatically maps to the following Go types:

| GraphQL type | Go type   |
----------------------------
| `Int`        | `int`     |
| `Float`      | `float64` |
| `String`     | `string`  |
| `Boolean`    | `bool`    |
| `ID`         | `string`  |

For custom scalars, or to map to different types, use the `bindings` option in [`genqlient.yaml`](genqlient.yaml).

[spec#scalar]: https://spec.graphql.org/draft/#sec-Scalars

### Custom scalars

Some schemas define custom scalars. You'll need to tell genqlient what types to use for those via the `bindings` option in `genqlient.yaml`, for example:

```yaml
bindings:
  DateTime:
    type: time.Time
```

The schema should define how custom scalars are encoded in JSON; you'll need to make sure the given type has the appropriate `MarshalJSON`/`UnmarshalJSON` or `json` tags. When using a third-party type, like `time.Time`, you can alternately define separate functions:

```yaml
bindings:
  DateTime:
    type: time.Time
    marshaler: "github.com/your/package.MarshalDateTime"
    unmarshaler: "github.com/your/package.UnmarshalDateTime"
```

See genqlient's integration tests for a full example: [types](../internal/testutil/types.go), [config](../internal/integration/genqlient.yaml).

To leave a custom scalar as raw JSON, map it to `encoding/json.RawMessage`:

```yaml
bindings:
  JSON:
    type: encoding/json.RawMessage
```

### Integer sizing


The GraphQL spec officially defines the `Int` type to be a [signed 32-bit integer](https://spec.graphql.org/draft/#sec-Int).  GraphQL clients and servers vary wildly in their enforcement of this; for example:
- [Apollo Server](https://github.com/apollographql/apollo-server/) explicitly checks that integers are at most 32 bits
- [gqlgen](https://github.com/99designs/gqlgen) by default allows any integer that fits in `int` (i.e. 64 bits on most platforms)
- [Apollo Client](https://github.com/apollographql/apollo-client) doesn't check (but implicitly is limited to 53 bits by JavaScript)
- [shurcooL/graphql](https://github.com/shurcooL/graphql) requires integers be passed as a `graphql.Int`, defined to be an `int32`

By default, genqlient maps GraphQL `Int`s to Go's `int`, meaning that on 64 bit systems there's no client-side restriction. This is convenient for most use cases, but means the client won't prevent you from passing a 64-bit integer to a server that will reject or truncate it.

If you prefer to limit integers to `int32`, you can set a binding in your `genqlient.yaml`:

```yaml
bindings:
  Int:
    type: int32
```

Or, you can bind it to any other type, perhaps one with size-checked constructors, similar to a custom scalar.

If your schema has a big-integer type, you can bind that similarly to other custom scalars:
```yaml
bindings:
  BigInt:
    type: math/big # or int64, or string, etc.
    # if you need custom marshaling
    marshaler: "github.com/path/to/package.MarshalBigInt"
    unmarshaler: "github.com/path/to/package.UnmarshalBigInt"
```

## Extensions

Some schemas/servers make use of GraphQL extensions, for example to add rate-limit information to responses. There are two ways to handle these in genqlient:

1. If you want to handle extensions in a general way for all queries (for example, to automatically retry after the rate-limit resets, you can do this in your [`graphql.Client` implementation](client_config.md#custom-clients).
2. To return response extensions directly in the generated helper functions (so that callers can decide what to do), set `use_extensions: true` in your [`genqlient.yaml`](genqlient.yaml).

## Hasura, Dgraph, and other generated schemas

Some GraphQL tools, like Hasura and Dgraph, generate large schemas automatically from non-GraphQL data (like database schemas). These schemas tend to be quite large and complex, and often run into trouble with GraphQL. See [#272](https://github.com/Khan/genqlient/issues/272) for discussion of how to use these tools with genqlient.
