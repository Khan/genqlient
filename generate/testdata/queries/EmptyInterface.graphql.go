package test

// Code generated by github.com/Khan/genqlient, DO NOT EDIT.

import (
	"github.com/Khan/genqlient/graphql"
)

type EmptyInterfaceResponse struct {
	GetJunk interface{} `json:"getJunk"`
}

func EmptyInterface(
	client graphql.Client,
) (*EmptyInterfaceResponse, error) {
	var retval EmptyInterfaceResponse
	err := client.MakeRequest(
		nil,
		"EmptyInterface",
		`
query EmptyInterface {
	getJunk
}
`,
		&retval,
		nil,
	)
	return &retval, err
}