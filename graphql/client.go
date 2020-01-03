package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

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

type payload struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

func (client *Client) MakeRequest(ctx context.Context, query string, retval interface{}, variables map[string]interface{}) error {
	body, err := json.Marshal(payload{
		Query:     query,
		Variables: variables,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(
		client.method,
		client.endpoint,
		bytes.NewReader(body))
	if err != nil {
		return err
	}

	req = req.WithContext(ctx)
	resp, err := client.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err = ioutil.ReadAll(resp.Body)
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
