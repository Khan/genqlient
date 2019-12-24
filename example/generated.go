package example

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/vektah/gqlparser/gqlerror"
)

type getViewerResponse = struct {
	Viewer struct {
		Name *string
	}
}

// TODO
func getViewer(ctx context.Context) (*getViewerResponse, error) {
	req, err := http.NewRequest(
		http.MethodPost,
		`https://api.github.com/graphql`,
		strings.NewReader(`
query getViewer {
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

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("returned error %v: %v", resp.Status, string(body))
	}

	var retval struct {
		Data   getViewerResponse `json:"data"`
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
