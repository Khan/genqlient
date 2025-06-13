# Using genqlient with GraphQL subscriptions

This document describes how to use genqlient to make GraphQL subscriptions. It assumes you already have the basic [client](./client_config.md) set up. Subscription support is fairly new; please report any bugs or missing features!

## Client setup

You will need to use a different client calling `graphql.NewClientUsingWebSocket`, passing as a parameter your own websocket client.

Here is how to configure your webSocket client to match the interfaces:

### Example using `github.com/gorilla/websocket`

```go
type MyDialer struct {
	*websocket.Dialer
}

func (md *MyDialer) DialContext(ctx context.Context, urlStr string, requestHeader http.Header) (graphql.WSConn, error) {
	conn, _, err := md.Dialer.DialContext(ctx, urlStr, requestHeader)
	return graphql.WSConn(conn), err
}
```

### Example using `golang.org/x/net/websocket`

```go
type MyDialer struct {
	dialer *net.Dialer
}

type MyConn struct {
	conn *websocket.Conn
}

func (c MyConn) ReadMessage() (messageType int, p []byte, err error) {
	if err := websocket.Message.Receive(c.conn, &p); err != nil {
		return websocket.UnknownFrame, nil, err
	}
	return messageType, p, err
}

func (c MyConn) WriteMessage(_ int, data []byte) error {
	err := websocket.Message.Send(c.conn, data)
	return err
}

func (c MyConn) Close() error {
	c.conn.Close()
	return nil
}

func (md *MyDialer) DialContext(ctx context.Context, urlStr string, requestHeader http.Header) (graphql.WSConn, error) {
	if md.dialer == nil {
		return nil, fmt.Errorf("nil dialer")
	}
	config, err := websocket.NewConfig(urlStr, "http://localhost")
	if err != nil {
		fmt.Println("Error creating WebSocket config:", err)
		return nil, err
	}
	config.Dialer = md.dialer
	config.Protocol = append(config.Protocol, "graphql-transport-ws")
	for key, values := range requestHeader {
		for _, value := range values {
			config.Header.Add(key, value)
		}
	}

	// Connect to the WebSocket server
	conn, err := websocket.DialConfig(config)
	if err != nil {
		return nil, err
	}
	return graphql.WSConn(MyConn{conn: conn}), err
}
```

## Making subscriptions

Once your websocket client matches the interfaces, you can get your `graphql.WebSocketClient` and listen in
a loop for incoming messages and errors:

```go
	graphqlClient := graphql.NewClientUsingWebSocket(
		"ws://localhost:8080/query",
		&MyDialer{Dialer: dialer},
		headers,
	)

	errChan, err := graphqlClient.Start(ctx)
	if err != nil {
		return
	}

	dataChan, subscriptionID, err := count(ctx, graphqlClient)
	if err != nil {
		return
	}

	defer graphqlClient.Close()
	for loop := true; loop; {
		select {
		case msg, more := <-dataChan:
			if !more {
				loop = false
				break
			}
			if msg.Data != nil {
				fmt.Println(msg.Data.Count)
			}
			if msg.Errors != nil {
				fmt.Println("error:", msg.Errors)
				loop = false
			}
		case err = <-errChan:
			return
		case <-time.After(time.Minute):
			err = graphqlClient.Unsubscribe(subscriptionID)
			loop = false
		}
	}
```

To change the websocket protocol from its default value `graphql-transport-ws`, add the following header before calling `graphql.NewClientUsingWebSocket()`:

```go
	headers.Add("Sec-WebSocket-Protocol", "graphql-ws")
```

## Authenticate subscriptions

Graphql allows to uthenticate subscriptions using HTTP headers (inside the http upgrade request) or using connection parameters (first message inside the websocket connection).
To authenticate using both methods, you need to add a `graphql.WebSocketOption` to the `graphql.NewClientUsingWebSocket` method.

### Example using HTTP headers

```go
graphql.NewClientUsingWebSocket(
	endpoint,
	&MyDialer{Dialer: dialer},
	graphql.WithConnectionParams(map[string]interface{}{
		"headers": map[string]string{
			"Authorization": "Bearer " + token.AccessToken,
		},
	}),
)
```

### Example using connection parameters

```go
graphql.NewClientUsingWebSocket(
	endpoint,
	&MyDialer{Dialer: dialer},
	graphql.WithWebsocketHeader(http.Header{
		"Authorization": []string{"Bearer " + token.AccessToken,},
	}),
)
```
