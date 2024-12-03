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

func TestMakeRequest_HTTPError(t *testing.T) {
	testCases := []struct {
		expectedError      *HTTPError
		serverResponseBody any
		name               string
		serverResponseCode int
	}{
		{
			name:               "plain_text_error",
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
			name:               "json_error_with_extensions",
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
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.serverResponseCode)
				err := json.NewEncoder(w).Encode(tc.serverResponseBody)
				if err != nil {
					t.Fatalf("Failed to write response: %v", err)
				}
			}))
			defer server.Close()

			client := NewClient(server.URL, server.Client())
			req := &Request{
				Query: "query { test }",
			}
			resp := &Response{}

			err := client.MakeRequest(context.Background(), req, resp)

			assert.Error(t, err)
			var httpErr *HTTPError
			assert.True(t, errors.As(err, &httpErr), "Error should be of type *HTTPError")
			assert.Equal(t, tc.expectedError, httpErr)
		})
	}
}

func TestMakeRequest_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]string{
				"test": "success",
			},
		})
		if err != nil {
			t.Fatalf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	req := &Request{
		Query: "query { test }",
	}
	resp := &Response{}

	err := client.MakeRequest(context.Background(), req, resp)

	assert.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"test": "success"}, resp.Data)
}
