package graphql

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMakeRequest_HTTPError(t *testing.T) {
	testCases := []struct {
		name               string
		serverResponseCode int
		serverResponseBody string
		expectedStatusCode int
		expectedErrorBody  string
	}{
		{
			name:               "400 Bad Request",
			serverResponseCode: http.StatusBadRequest,
			serverResponseBody: "Bad Request",
			expectedStatusCode: http.StatusBadRequest,
			expectedErrorBody:  "Bad Request",
		},
		{
			name:               "429 Too Many Requests",
			serverResponseCode: http.StatusTooManyRequests,
			serverResponseBody: "Rate limit exceeded",
			expectedStatusCode: http.StatusTooManyRequests,
			expectedErrorBody:  "Rate limit exceeded",
		},
		{
			name:               "500 Internal Server Error",
			serverResponseCode: http.StatusInternalServerError,
			serverResponseBody: "Internal Server Error",
			expectedStatusCode: http.StatusInternalServerError,
			expectedErrorBody:  "Internal Server Error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.serverResponseCode)
				w.Write([]byte(tc.serverResponseBody))
			}))
			defer server.Close()

			client := NewClient(server.URL, server.Client())
			req := &Request{
				Query: "query { test }",
			}
			resp := &Response{}

			err := client.MakeRequest(context.Background(), req, resp)

			assert.Error(t, err)
			httpErr, ok := err.(*HTTPError)
			assert.True(t, ok, "Error should be of type *HTTPError")
			assert.Equal(t, tc.expectedStatusCode, httpErr.StatusCode)
			assert.Equal(t, tc.expectedErrorBody, httpErr.Body)
		})
	}
}

func TestMakeRequest_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]string{
				"test": "success",
			},
		})
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
