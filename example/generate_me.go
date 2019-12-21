package example

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
)

type GetViewerResponse = struct {
	Viewer struct {
		Name string `json:"name"`
	} `json:"viewer"`
}

// GetViewer gets the current user's name.
func GetViewer(ctx context.Context) (*GetViewerResponse, error) {
	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		`https://api.github.com/graphql`,
		strings.NewReader(`
"GetViewer gets the current user's name."
query GetViewer {
  Viewer: viewer {
    Name: name
  }
}
`))
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	retval := GetViewerResponse{}
	err = json.Unmarshal(body, &retval)
	if err != nil {
		return nil, err
	}

	return &retval, nil
}
