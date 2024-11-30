package graphql

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMakeRequest_HTTPError(t *testing.T) {
	testCases := []struct {
		name               string
		serverResponseBody string
		expectedErrorBody  string
		serverResponseCode int
		expectedStatusCode int
	}{
		{
			name:               "400 Bad Request",
			serverResponseBody: "Bad Request",
			expectedErrorBody:  "Bad Request",
			serverResponseCode: http.StatusBadRequest,
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			name:               "429 Too Many Requests",
			serverResponseBody: "Rate limit exceeded",
			expectedErrorBody:  "Rate limit exceeded",
			serverResponseCode: http.StatusTooManyRequests,
			expectedStatusCode: http.StatusTooManyRequests,
		},
		{
			name:               "500 Internal Server Error",
			serverResponseBody: "Internal Server Error",
			expectedErrorBody:  "Internal Server Error",
			serverResponseCode: http.StatusInternalServerError,
			expectedStatusCode: http.StatusInternalServerError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.serverResponseCode)
				_, err := w.Write([]byte(tc.serverResponseBody))
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
			assert.Equal(t, tc.expectedStatusCode, httpErr.StatusCode)
			assert.Equal(t, tc.expectedErrorBody, httpErr.Body)
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
