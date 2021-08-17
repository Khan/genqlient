package server

import (
	"context"
	"net/http/httptest"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
)

func intptr(v int) *int { return &v }

var users = []*User{
	{ID: "1", Name: "Yours Truly", LuckyNumber: intptr(17)},
	{ID: "2", Name: "Raven", LuckyNumber: intptr(-1)},
}

func userByID(id string) *User {
	for _, user := range users {
		if id == user.ID {
			return user
		}
	}
	return nil
}

func (r *queryResolver) Me(ctx context.Context) (*User, error) {
	return userByID("1"), nil
}

func (r *queryResolver) User(ctx context.Context, id string) (*User, error) {
	return userByID(id), nil
}

func RunServer() *httptest.Server {
	gqlgenServer := handler.New(NewExecutableSchema(Config{Resolvers: &resolver{}}))
	gqlgenServer.AddTransport(transport.POST{})
	return httptest.NewServer(gqlgenServer)
}

type (
	resolver      struct{}
	queryResolver struct{}
)

func (r *resolver) Query() QueryResolver { return &queryResolver{} }

//go:generate go run github.com/99designs/gqlgen
