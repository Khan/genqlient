package testutil

import (
	"context"
	"time"

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

const dateFormat = "2006-01-02"

func MarshalDate(t *time.Time) ([]byte, error) {
	if t == nil || t.IsZero() {
		return []byte("null"), nil
	}
	return []byte(`"` + t.Format(dateFormat) + `"`), nil
}

func UnmarshalDate(b []byte, t *time.Time) error {
	// (modified from time.Time.UnmarshalJSON)
	var err error
	*t, err = time.Parse(`"`+dateFormat+`"`, string(b))
	return err
}
