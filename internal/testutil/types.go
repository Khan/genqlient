package testutil

import (
	"context"

	"github.com/Khan/genqlient/graphql"
)

type ID string

type Pokemon struct {
	Species string `json:"species"`
	Level   int    `json:"level"`
}

func (p Pokemon) Battle(q Pokemon) bool {
	return p.Level > q.Level
}

type MyContext interface {
	context.Context

	MyMethod()
}

func GetClientFromNowhere() (graphql.Client, error)                    { return nil, nil }
func GetClientFromContext(ctx context.Context) (graphql.Client, error) { return nil, nil }
func GetClientFromMyContext(ctx MyContext) (graphql.Client, error)     { return nil, nil }
