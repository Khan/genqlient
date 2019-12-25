package graphql

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/vektah/gqlparser/gqlerror"
)

type Client struct {
	endpoint   string
	method     string
	httpClient *http.Client
}

func NewClient(endpoint string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{endpoint, http.MethodPost, httpClient}
}

func (client *Client) MakeRequest(ctx context.Context, query string, retval interface{}) error {
	req, err := http.NewRequest(
		client.method,
		client.endpoint,
		strings.NewReader(query))
	if err != nil {
		return err
	}

	req = req.WithContext(ctx)
	resp, err := client.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("returned error %v: %v", resp.Status, string(body))
	}

	var dataAndErrors struct {
		Data   json.RawMessage `json:"data"`
		Errors gqlerror.List   `json:"errors"`
	}

	err = json.Unmarshal(body, &dataAndErrors)
	if err != nil {
		return err
	}

	if len(dataAndErrors.Errors) > 0 {
		return dataAndErrors.Errors
	}

	return json.Unmarshal(dataAndErrors.Data, &retval)
}
