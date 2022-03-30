package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/vektah/gqlparser/v2/gqlerror"
)

// Client is the interface that the generated code calls into to actually make
// requests.
//
// Unstable: This interface is likely to change before v1.0, see #19.  Creating
// a client with NewClient will remain the same.
type Client interface {
	// MakeRequest must make a request to the client's GraphQL API.
	//
	// ctx is the context that should be used to make this request.  If context
	// is disabled in the genqlient settings, this will be set to
	// context.Background().
	//
	// req is the Request object that should be sent to the GraphQL server.
	// MakeRequest is marshalling this into a byte sequence readable by
	// the API server.
	//
	// resp is the Response object that will be used to store the returned
	// data to. MakeRequest will try to unmarshal the data returend by the GraphQL
	// server. Thus it is expected, that you set the Data field to your expected
	// Datatype. example:
	//     var data complexDataStruct
	//     resp := &graphql.Response{Data: &data}
	// In case errors are returned in the response, Makerequest will return these.
	// Extensions are added as well, if sent by the server, during regular unmarshalling
	MakeRequest(
		ctx context.Context,
		req *Request,
		resp *Response,
	) error
}

type client struct {
	httpClient Doer
	endpoint   string
	method     string
}

// NewClient returns a Client which makes requests to the given endpoint,
// suitable for most users.
//
// The client makes POST requests to the given GraphQL endpoint using standard
// GraphQL HTTP-over-JSON transport.  It will use the given http client, or
// http.DefaultClient if a nil client is passed.
//
// The typical method of adding authentication headers is to wrap the client's
// Transport to add those headers.  See example/caller.go for an example.
func NewClient(endpoint string, httpClient Doer) Client {
	if httpClient == nil || httpClient == (*http.Client)(nil) {
		httpClient = http.DefaultClient
	}
	return &client{httpClient, endpoint, http.MethodPost}
}

// Doer encapsulates the methods from *http.Client needed by Client.
// The methods should have behavior to match that of *http.Client
// (or mocks for the same).
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// Request contains all the values required to build queries executed by
// the graphql.Client.
//
// Query is the literal string representing the GraphQL query, e.g.
// `query myQuery { myField }`.
// Variables contains a JSON-marshalable value containing the variables
// to be sent along with the query, or may be nil if there are none.
// Typically, GraphQL APIs will  accept a JSON payload of the form
//	{"query": "query myQuery { ... }", "variables": {...}}`
// OpName is only required if there are multiple queries in the document,
// but we set it unconditionally, because that's easier.
type Request struct {
	Query     string      `json:"query"`
	Variables interface{} `json:"variables,omitempty"`
	OpName    string      `json:"operationName"`
}

// Response that contains data returned by the GraphQL API.
//
// Typically, GraphQL APIs will return a JSON payload of the form
//	{"data": {...}, "errors": {...}}, additionally it can contain a key
// named "extensions", that might hold GraphQL protocol extensions.
// Extensions and Errors are optional, depending on the values
// returned by the Request.
type Response struct {
	Data       interface{}            `json:"data"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
	Errors     gqlerror.List          `json:"errors,omitempty"`
}

func (c *client) MakeRequest(ctx context.Context, req *Request, resp *Response) error {
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequest(
		c.method,
		c.endpoint,
		bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	if ctx != nil {
		httpReq = httpReq.WithContext(ctx)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		var respBody []byte
		respBody, err = io.ReadAll(httpResp.Body)
		if err != nil {
			respBody = []byte(fmt.Sprintf("<unreadable: %v>", err))
		}
		return fmt.Errorf("returned error %v: %s", httpResp.Status, respBody)
	}

	err = json.NewDecoder(httpResp.Body).Decode(resp)
	if err != nil {
		return err
	}
	if len(resp.Errors) > 0 {
		return resp.Errors
	}
	return nil
}
