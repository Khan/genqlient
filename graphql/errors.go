package graphql

import "fmt"

// HTTPError represents an HTTP error with status code and response body.
type HTTPError struct {
	Body       string
	StatusCode int
}

// Error implements the error interface for HTTPError.
func (e *HTTPError) Error() string {
	return fmt.Sprintf("returned error %v: %s", e.StatusCode, e.Body)
}
