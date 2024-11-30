# Configuring and using the genqlient client

This document describes common patterns for using the genqlient client at runtime. For full client reference documentation, see the [godoc].

[godoc]: https://pkg.go.dev/github.com/Khan/genqlient/graphql

## Creating a client

For most users, just call [`graphql.NewClient`][godoc#NewClient] to get a `graphql.Client`, which you can then pass to genqlient's generated functions. For example, `graphql.NewClient("https://your.api.example/path", http.DefaultClient)` will call an API at the given URL in a fashion compatible with most GraphQL servers.

For example (see the [getting started docs](INTRODUCTION.md) for the full setup):

```go
ctx := context.Background()
client := graphql.NewClient("https://api.github.com/graphql", http.DefaultClient)
resp, err := getUser(ctx, client, "benjaminjkraft")
fmt.Println(resp.User.Name, err)
```

You can pass the client around however you like to inject dependencies, such as via a global variable, context value, or [fancy typed context][kacontext].

[godoc#NewClient]: https://pkg.go.dev/github.com/Khan/genqlient/graphql#NewClient
[kacontext]: https://blog.khanacademy.org/statically-typed-context-in-go/

### Authentication and other headers

To use an API requiring authentication, you can customize the HTTP client passed to [`graphql.NewClient`][godoc#NewClient] to add whatever headers you need. The usual way to do this is to wrap the client's `Transport`:

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

The same method works for passing other HTTP headers, like [`traceparent`](https://www.w3.org/TR/trace-context/). To set a request-dependent header, the `RoundTrip` method has access to the full request, including the context from `req.Context()`. For more on wrapping HTTP clients, see [this post](https://dev.to/stevenacoffman/tripperwares-http-client-middleware-chaining-roundtrippers-3o00).

### GET requests

To use GET instead of POST requests, use [`graphql.NewClientUsingGet`][godoc#NewClientUsingGet) to create a client that puts the request in GET query parameters, compatible with many GraphQL servers. For example:
```go
ctx := context.Background()
client := graphql.NewClientUsingGet("https://api.github.com/graphql", http.DefaultClient)
resp, err := getUser(ctx, client, "benjaminjkraft")
fmt.Println(resp.User.Name, err)
```

The request will be sent via an HTTP GET request, with the query, operation name and variables encoded in the URL, like so:
```
https://api.github.com/graphql?operationName%3DgetUser%26query%3D%0Aquery%20getUser(%24login%3A%20String!)%20%7B%0A%20%20user(login%3A%20%24login)%20%7B%0A%20%20%20%20name%0A%20%20%7D%0A%7D%0A%26variables%3D%7B%22login%22%3A%22benjaminjkraft%22%7D
```

This is useful for caching requests in a CDN or browser cache. It's not recommended for requests containing sensitive data. This client does not support mutations, and will return an error if used for a mutation.

[godoc#NewClientUsingGet]: https://pkg.go.dev/github.com/Khan/genqlient/graphql#NewClientUsingGet

### Custom clients

The genqlient client is an interface; you may define your own implementation. This could wrap the ordinary client to handle GraphQL extensions or set query-specific headers; or start from scratch to use a custom transport. For details, see the [documentation][godoc#Client].

[godoc#Client]: https://pkg.go.dev/github.com/Khan/genqlient/graphql#Client

## Testing

### Testing code that uses genqlient

Testing code that uses genqlient typically involves passing in a special HTTP client that does what you want, similar to authentication.  For example, you might write a client whose `RoundTrip` returns a fixed response, constructed with [`httptest`].  Or, you can use `httptest` to start up a temporary server, and point genqlient at that.  Many third-party packages provide support for this sort of thing; genqlient should work with any HTTP-level mocking that can expose a regular `http.Client`.

For an example, genqlient's own integration tests use both approaches:
- we [set up a simple GraphQL server](../internal/integration/server/server.go) using [`gqlgen`][gqlgen] and [`httptest`][httptest], and run requests against that
- we also [wrap the HTTP client](../internal/integration/roundtrip.go) to do extra assertions about each request and response (to check the marshaling and unmarshaling logic).

[gqlgen]: https://gqlgen.com/
[httptest]: https://pkg.go.dev/net/http/httptest

### Testing servers

If you want, you can use genqlient to test your GraphQL APIs; as with mocking you can point genqlient at anything that exposes an ordinary HTTP endpoint or a custom `http.Client`. However, at Khan Academy we've found that genqlient usually isn't the best client for testing: for example, manually constructing values of genqlient's response types gets cumbersome when interfaces or fragments are involved. Instead, we prefer to use a lightweight (and weakly-typed) client for that, and may separately open-source ours in the future.

## Response objects

Each genqlient-generated helper function returns a struct whose type corresponds to the query result. For example, given a simple query:

```graphql
query getUser($login: String!) {
  user(login: $login) {
    name
  }
}
```

genqlient will generate something like the following:

```go
func getUser(...) (*getUserResponse, error) { ... }

type getUserResponse struct {
	User getUserUser
}

type getUserUser struct {
	Name string
}
```

For more on accessing response objects for interfaces and fragments, see the [operations documentation](operations.md#interfaces).

### Handling errors

In addition to the response-struct, each genqlient-generated helper function returns an error. The error may be [`As`-able][As] to one of the following:

- [`gqlerror.List`][gqlerror], if the request returns a valid GraphQL response containing errors; in this case the struct may be partly-populated 
- [`graphql.HTTPError`][HTTPError], if there was a valid but non-200 HTTP response
- another error (e.g. a [`*url.Error`][urlError])

In case of a GraphQL error, the response-struct may be partly-populated (if one field failed but another was computed successfully). In other cases it will be blank, but it will always be initialized (never nil), even on error.

[As]: https://pkg.go.dev/errors#As
[gqlerror]: https://pkg.go.dev/github.com/vektah/gqlparser/v2/gqlerror#List
[HTTPError]: https://pkg.go.dev/github.com/Khan/genqlient/graphql#HTTPError
[urlError]: https://pkg.go.dev/net/url#Error

For example, you might do one of the following:
```go
// return both error and field:
resp, err := getUser(...)
return resp.User.Name, err

// handle different errors differently:
resp, err := getUser(...)
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

### Marshaling

All genqlient-generated types support both JSON-marshaling and unmarshaling, which can be useful for putting them in a cache, inspecting them by hand, using them in mocks (although this is [not recommended](#testing-servers)), or anything else you can do with JSON.  It's not guaranteed that marshaling a genqlient type will produce the exact GraphQL input -- we try to get as close as we can but there are some limitations around Go zero values -- but unmarshaling again should produce the value genqlient returned.  That is:

```go
resp, err := MyQuery(...)
// not guaranteed to match what the server sent (but close):
b, err := json.Marshal(resp)
// guaranteed to match resp:
var respAgain MyQueryResponse
err := json.Unmarshal(b, &resp)
```

