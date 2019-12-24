package example

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/vektah/gqlparser/gqlerror"
)

type GetViewerResponse = struct {
	Viewer struct {
		Name *string
	}
}

// TODO
func GetViewer(ctx context.Context) (*GetViewerResponse, error) {
	req, err := http.NewRequest(
		http.MethodPost,
		`https://api.github.com/graphql`,
		strings.NewReader(`
query GetViewer {
	Viewer: viewer {
		Name: name
	}
}
`))
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var retval struct {
		Data   GetViewerResponse `json:"data"`
		Errors gqlerror.List     `json:"errors"`
	}
	err = json.Unmarshal(body, &retval)
	if err != nil {
		return nil, err
	}

	if len(retval.Errors) > 0 {
		return nil, retval.Errors
	}

	return &retval.Data, nil
}
