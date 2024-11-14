package integration

// Machinery for integration tests to round-trip check the JSON-marshalers and
// unmarshalers we generate.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Khan/genqlient/graphql"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

// lastResponseTransport is an HTTP transport that keeps track of the last response
// that passed through it.
type lastResponseTransport struct {
	wrapped          http.RoundTripper
	lastResponseBody []byte
}

func (t *lastResponseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.wrapped.RoundTrip(req)
	if err != nil {
		return resp, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, fmt.Errorf("roundtrip failed: unreadable body: %w", err)
	}
	t.lastResponseBody = body
	// Restore the body for the next reader:
	resp.Body = io.NopCloser(bytes.NewBuffer(body))
	return resp, err
}

// roundtripClient is a graphql.Client that checks that
//
//	unmarshal(marshal(req)) == req && marshal(unmarshal(resp)) == resp
//
// for each request it processes.
type roundtripClient struct {
	wrapped   graphql.Client
	wsWrapped graphql.WebSocketClient
	transport *lastResponseTransport
	t         *testing.T
}

// Put JSON in a stable and human-readable format.
func (c *roundtripClient) formatJSON(b []byte) []byte {
	// We don't care about key ordering, so do another roundtrip through
	// interface{} to drop that.
	var parsed interface{}
	err := json.Unmarshal(b, &parsed)
	if err != nil {
		c.t.Fatal(err)
	}

	// When marshaling, add indents to make things human-readable.
	b, err = json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		c.t.Fatal(err)
	}
	return b
}

func (c *roundtripClient) roundtripResponse(resp interface{}) {
	var graphqlResponse struct {
		Data json.RawMessage `json:"data"`
	}
	err := json.Unmarshal(c.transport.lastResponseBody, &graphqlResponse)
	if err != nil {
		c.t.Error(err)
		return
	}
	body := c.formatJSON(graphqlResponse.Data)

	// resp is constructed to be unmarshal(body), so just use it
	bodyAgain, err := json.Marshal(resp)
	if err != nil {
		c.t.Error(err)
		return
	}
	bodyAgain = c.formatJSON(bodyAgain)

	assert.Equal(c.t, string(body), string(bodyAgain))
}

func (c *roundtripClient) MakeRequest(ctx context.Context, req *graphql.Request, resp *graphql.Response) error {
	// TODO(benkraft): Also check the variables round-trip.  This is a bit less
	// important since most of the code is the same (and input types are
	// strictly simpler), and a bit hard to do because when asserting about
	// structs we need to worry about things like equality of time.Time values.
	err := c.wrapped.MakeRequest(ctx, req, resp)
	if err != nil {
		return err
	}
	c.roundtripResponse(resp.Data)
	return nil
}

func (c *roundtripClient) Start(ctx context.Context) (errChan chan error, err error) {
	return c.wsWrapped.Start(ctx)
}

func (c *roundtripClient) Close() error {
	return c.wsWrapped.Close()
}

func (c *roundtripClient) Subscribe(req *graphql.Request, interfaceChan interface{}, forwardDataFunc graphql.ForwardDataFunction) (string, error) {
	return c.wsWrapped.Subscribe(req, interfaceChan, forwardDataFunc)
}

func (c *roundtripClient) Unsubscribe(subscriptionID string) error {
	return c.wsWrapped.Unsubscribe(subscriptionID)
}

func newRoundtripClients(t *testing.T, endpoint string) []graphql.Client {
	return []graphql.Client{newRoundtripClient(t, endpoint), newRoundtripGetClient(t, endpoint)}
}

func newRoundtripClient(t *testing.T, endpoint string) graphql.Client {
	transport := &lastResponseTransport{wrapped: http.DefaultTransport}
	httpClient := &http.Client{Transport: transport}
	return &roundtripClient{
		wrapped:   graphql.NewClient(endpoint, httpClient),
		transport: transport,
		t:         t,
	}
}

func newRoundtripGetClient(t *testing.T, endpoint string) graphql.Client {
	transport := &lastResponseTransport{wrapped: http.DefaultTransport}
	httpClient := &http.Client{Transport: transport}
	return &roundtripClient{
		wrapped:   graphql.NewClientUsingGet(endpoint, httpClient),
		transport: transport,
		t:         t,
	}
}

type MyDialer struct {
	*websocket.Dialer
}

func (md *MyDialer) DialContext(ctx context.Context, urlStr string, requestHeader http.Header) (graphql.WSConn, error) {
	conn, resp, err := md.Dialer.DialContext(ctx, urlStr, requestHeader)
	resp.Body.Close()
	return graphql.WSConn(conn), err
}

func newRoundtripWebSocketClient(t *testing.T, endpoint string, connectionParams map[string]interface{}) graphql.WebSocketClient {
	dialer := websocket.DefaultDialer
	if !strings.HasPrefix(endpoint, "ws") {
		_, address, _ := strings.Cut(endpoint, "://")
		endpoint = "ws://" + address
	}
	return &roundtripClient{
		wsWrapped: graphql.NewClientUsingWebSocketWithConnectionParams(
			endpoint,
			&MyDialer{Dialer: dialer},
			nil,
			connectionParams,
		),
		t: t,
	}
}
