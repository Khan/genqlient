package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/gorilla/websocket"
)

type MyDialer struct {
	*websocket.Dialer
}

func (md *MyDialer) DialContext(ctx context.Context, urlStr string, requestHeader http.Header) (graphql.WSConn, *http.Response, error) {
	conn, resp, err := md.Dialer.DialContext(ctx, urlStr, requestHeader)
	return graphql.WSConn(conn), resp, err
}

func main() {
	var err error
	defer func() {
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}()

	key := os.Getenv("GITHUB_TOKEN")
	if key == "" {
		err = fmt.Errorf("must set GITHUB_TOKEN=<github token>")
		return
	}

	dialer := websocket.DefaultDialer
	headers := http.Header{}
	headers.Add("Authorization", "bearer "+key)

	graphqlClient := graphql.NewClientUsingWebSocket(
		"wss://api.github.com/graphql",
		&MyDialer{Dialer: dialer},
		headers,
	)

	respChan, errChan, err := count(context.Background(), graphqlClient)
	if err != nil {
		return
	}
	defer graphqlClient.CloseWebSocket()
	for loop := true; loop; {
		select {
		case msg, more := <-respChan:
			if !more {
				loop = false
				break
			}
			if msg.Data != nil {
				fmt.Println(msg.Data)
			}
			if msg.Errors != nil {
				fmt.Println("error:", msg.Errors)
				loop = false
			}
		case err = <-errChan:
			return
		case <-time.After(time.Second * 5):
			loop = false
		}
	}
}

//go:generate go run github.com/Khan/genqlient genqlient.yaml
