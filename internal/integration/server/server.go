package server

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strconv"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
)

func strptr(v string) *string { return &v }
func intptr(v int) *int       { return &v }

var users = []*User{
	{
		ID: "1", Name: "Yours Truly", LuckyNumber: intptr(17),
		Birthdate:   strptr("2025-01-01"),
		Hair:        &Hair{Color: strptr("Black")},
		GreatScalar: strptr("cool value"),
	},
	{ID: "2", Name: "Raven", LuckyNumber: intptr(-1), Hair: nil, GreatScalar: strptr("cool value")},
}

func init() {
	users[0].Friends = []*User{users[1]} // (obviously a lie, but)
	users[1].Friends = users             // try to crash the system
}

var animals = []*Animal{
	{
		ID: "3", Name: "Fido", Species: SpeciesDog, Owner: userByID("1"),
		Hair: &BeingsHair{HasHair: true},
	},
	{
		ID: "4", Name: "Old One", Species: SpeciesCoelacanth, Owner: nil,
		Hair: &BeingsHair{HasHair: false},
	},
}

func userByID(id string) *User {
	for _, user := range users {
		if id == user.ID {
			return user
		}
	}
	return nil
}

func usersByBirthdates(dates []string) []*User {
	var retval []*User
	for _, date := range dates {
		for _, user := range users {
			if user.Birthdate != nil && *user.Birthdate == date {
				retval = append(retval, user)
			}
		}
	}
	return retval
}

func beingByID(id string) Being {
	for _, user := range users {
		if id == user.ID {
			return user
		}
	}
	for _, animal := range animals {
		if id == animal.ID {
			return animal
		}
	}
	return nil
}

func getNewID() string {
	maxID := 0
	for _, user := range users {
		intID, _ := strconv.Atoi(user.ID)
		if intID > maxID {
			maxID = intID
		}
	}
	for _, animal := range animals {
		intID, _ := strconv.Atoi(animal.ID)
		if intID > maxID {
			maxID = intID
		}
	}
	newID := maxID + 1
	return strconv.Itoa(newID)
}

func (r *queryResolver) Me(ctx context.Context) (*User, error) {
	return userByID("1"), nil
}

func (r *queryResolver) User(ctx context.Context, id *string) (*User, error) {
	if id == nil {
		return userByID("1"), nil
	}
	return userByID(*id), nil
}

func (r *queryResolver) Being(ctx context.Context, id string) (Being, error) {
	return beingByID(id), nil
}

func (r *queryResolver) Beings(ctx context.Context, ids []string) ([]Being, error) {
	ret := make([]Being, len(ids))
	for i, id := range ids {
		ret[i] = beingByID(id)
	}
	return ret, nil
}

func (r *queryResolver) LotteryWinner(ctx context.Context, number int) (Lucky, error) {
	for _, user := range users {
		if user.LuckyNumber != nil && *user.LuckyNumber == number {
			return user, nil
		}
	}
	return nil, nil
}

func (r *queryResolver) UsersBornOn(ctx context.Context, date string) ([]*User, error) {
	return usersByBirthdates([]string{date}), nil
}

func (r *queryResolver) UsersBornOnDates(ctx context.Context, dates []string) ([]*User, error) {
	return usersByBirthdates(dates), nil
}

func (r *queryResolver) UserSearch(ctx context.Context, birthdate *string, id *string) ([]*User, error) {
	switch {
	case birthdate == nil && id != nil:
		return []*User{userByID(*id)}, nil
	case birthdate != nil && id == nil:
		return usersByBirthdates([]string{*birthdate}), nil
	default:
		return nil, fmt.Errorf("need exactly one of birthdate or id")
	}
}

func (r *queryResolver) Fail(ctx context.Context) (*bool, error) {
	f := true
	return &f, fmt.Errorf("oh no")
}

func (m mutationResolver) CreateUser(ctx context.Context, input NewUser) (*User, error) {
	newUser := User{ID: getNewID(), Name: input.Name, Friends: []*User{}}
	users = append(users, &newUser)
	return &newUser, nil
}

func (s *subscriptionResolver) Count(ctx context.Context) (<-chan int, error) {
	respChan := make(chan int, 1)
	go func(respChan chan int) {
		defer close(respChan)
		counter := 0
		for {
			if counter == 10 {
				return
			}
			respChan <- counter
			counter++
			time.Sleep(100 * time.Millisecond)
		}
	}(respChan)
	return respChan, nil
}

func (s *subscriptionResolver) CountAuthorized(ctx context.Context) (<-chan int, error) {
	if getAuthToken(ctx) != "authorized-user-token" {
		return nil, fmt.Errorf("unauthorized")
	}

	return s.Count(ctx)
}

const AuthKey = "authToken"

type (
	authTokenCtxKey struct{}
)

func withAuthToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, authTokenCtxKey{}, token)
}

func getAuthToken(ctx context.Context) string {
	if tkn, ok := ctx.Value(authTokenCtxKey{}).(string); ok {
		return tkn
	}
	return ""
}

func RunServer() *httptest.Server {
	gqlgenServer := handler.New(NewExecutableSchema(Config{Resolvers: &resolver{}}))
	gqlgenServer.AddTransport(transport.POST{})
	gqlgenServer.AddTransport(transport.GET{})

	gqlgenServer.AddTransport(transport.Websocket{
		InitFunc: func(ctx context.Context, initPayload transport.InitPayload) (context.Context, *transport.InitPayload, error) {
			if authToken, ok := initPayload[AuthKey].(string); ok && authToken != "" {
				ctx = withAuthToken(ctx, authToken)
			}
			return ctx, &initPayload, nil
		},
	})

	gqlgenServer.AroundResponses(func(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
		graphql.RegisterExtension(ctx, "foobar", "test")
		return next(ctx)
	})
	return httptest.NewServer(gqlgenServer)
}

type (
	resolver             struct{}
	queryResolver        struct{}
	mutationResolver     struct{}
	subscriptionResolver struct{}
)

func (r *resolver) Mutation() MutationResolver {
	return &mutationResolver{}
}

func (r *resolver) Query() QueryResolver { return &queryResolver{} }

func (r *resolver) Subscription() SubscriptionResolver {
	return &subscriptionResolver{}
}

//go:generate go run github.com/99designs/gqlgen@v0.17.35
