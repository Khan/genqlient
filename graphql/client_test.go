package graphql

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

func makeServer(t *testing.T, responseCode int, responseBody any) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(responseCode)
		err := json.NewEncoder(w).Encode(responseBody)
		if err != nil {
			t.Fatalf("Failed to write response: %v", err)
		}
	}))
}

func makeRequest(server *httptest.Server) (*Response, error) {
	client := NewClient(server.URL, server.Client())
	req := &Request{Query: "query { test }"}
	resp := &Response{}

	err := client.MakeRequest(context.Background(), req, resp)
	return resp, err
}

func TestMakeRequestHTTPError(t *testing.T) {
	testCases := []struct {
		expectedError      *HTTPError
		serverResponseBody any
		name               string
		serverResponseCode int
	}{
		{
			name:               "PlainTextError",
			serverResponseCode: http.StatusBadRequest,
			serverResponseBody: "Bad Request",
			expectedError: &HTTPError{
				Response: Response{
					Errors: gqlerror.List{
						&gqlerror.Error{
							Message: "\"Bad Request\"\n",
						},
					},
				},
				StatusCode: http.StatusBadRequest,
			},
		},
		{
			name:               "JSONErrorWithExtensions",
			serverResponseCode: http.StatusTooManyRequests,
			serverResponseBody: Response{
				Errors: gqlerror.List{
					&gqlerror.Error{
						Message: "Rate limit exceeded",
						Extensions: map[string]interface{}{
							"code": "RATE_LIMIT_EXCEEDED",
						},
					},
				},
			},
			expectedError: &HTTPError{
				Response: Response{
					Errors: gqlerror.List{
						&gqlerror.Error{
							Message: "Rate limit exceeded",
							Extensions: map[string]interface{}{
								"code": "RATE_LIMIT_EXCEEDED",
							},
						},
					},
				},
				StatusCode: http.StatusTooManyRequests,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := makeServer(t, tc.serverResponseCode, tc.serverResponseBody)
			defer server.Close()
			_, err := makeRequest(server)

			assert.Error(t, err)
			var httpErr *HTTPError
			assert.True(t, errors.As(err, &httpErr), "Error should be of type *HTTPError")
			assert.Equal(t, tc.expectedError, httpErr)
		})
	}
}

func TestMakeRequestHTTPErrors(t *testing.T) {
	server := makeServer(t, http.StatusOK, Response{
		Errors: gqlerror.List{&gqlerror.Error{Message: "Rate limit exceeded"}},
	})
	defer server.Close()
	_, err := makeRequest(server)

	assert.Error(t, err)
	var gqlErr gqlerror.List
	assert.True(t, errors.As(err, &gqlErr), "Error should be of type *gqlerror.List")
	assert.Equal(t, gqlerror.List{&gqlerror.Error{Message: "Rate limit exceeded"}}, gqlErr)
}

func TestMakeRequestSuccess(t *testing.T) {
	server := makeServer(t, http.StatusOK, map[string]interface{}{
		"data": map[string]string{"test": "success"},
	})
	defer server.Close()
	resp, err := makeRequest(server)

	assert.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"test": "success"}, resp.Data)
}
