package test

// Code generated by github.com/Khan/genql, DO NOT EDIT.

import (
	"github.com/Khan/genql/graphql"
)

type TypeNameQueryResponse struct {
	User TypeNameQueryUser `json:"user"`
}

type TypeNameQueryUser struct {
	Typename string `json:"__typename"`
	Id       string `json:"id"`
}

func TypeNameQuery(
	client *graphql.Client,
) (*TypeNameQueryResponse, error) {
	var retval TypeNameQueryResponse
	err := client.MakeRequest(
		nil,
		`
query TypeNameQuery {
	user {
		__typename
		id
	}
}
`,
		&retval,
		nil,
	)
	return &retval, err
}