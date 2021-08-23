// Package integration contains genqlient's integration tests, which run
// against a real server (defined in internal/integration/server/server.go).
//
// These are especially important for cases where we generate nontrivial logic,
// such as JSON-unmarshaling.
package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Khan/genqlient/graphql"
	"github.com/Khan/genqlient/internal/integration/server"
)

func TestSimpleQuery(t *testing.T) {
	_ = `# @genqlient
	query simpleQuery { me { id name luckyNumber } }`

	ctx := context.Background()
	server := server.RunServer()
	defer server.Close()
	client := graphql.NewClient(server.URL, http.DefaultClient)

	resp, err := simpleQuery(ctx, client)
	require.NoError(t, err)

	assert.Equal(t, "1", resp.Me.Id)
	assert.Equal(t, "Yours Truly", resp.Me.Name)
	assert.Equal(t, 17, resp.Me.LuckyNumber)
}

func TestVariables(t *testing.T) {
	_ = `# @genqlient
	query queryWithVariables($id: ID!) { user(id: $id) { id name luckyNumber } }`

	ctx := context.Background()
	server := server.RunServer()
	defer server.Close()
	client := graphql.NewClient(server.URL, http.DefaultClient)

	resp, err := queryWithVariables(ctx, client, "2")
	require.NoError(t, err)

	assert.Equal(t, "2", resp.User.Id)
	assert.Equal(t, "Raven", resp.User.Name)
	assert.Equal(t, -1, resp.User.LuckyNumber)

	resp, err = queryWithVariables(ctx, client, "374892379482379")
	require.NoError(t, err)

	assert.Zero(t, resp.User)
}

func TestInterfaceNoFragments(t *testing.T) {
	_ = `# @genqlient
	query queryWithInterfaceNoFragments($id: ID!) {
		being(id: $id) { id name }
		me { id name }
	}`

	ctx := context.Background()
	server := server.RunServer()
	defer server.Close()
	client := graphql.NewClient(server.URL, http.DefaultClient)

	resp, err := queryWithInterfaceNoFragments(ctx, client, "1")
	require.NoError(t, err)

	// We should get the following response:
	//	me: User{Id: 1, Name: "Yours Truly"},
	//	being: User{Id: 1, Name: "Yours Truly"},

	assert.Equal(t, "1", resp.Me.Id)
	assert.Equal(t, "Yours Truly", resp.Me.Name)

	// Check fields both via interface and via type-assertion:
	assert.Equal(t, "User", resp.Being.GetTypename())
	assert.Equal(t, "1", resp.Being.GetId())
	assert.Equal(t, "Yours Truly", resp.Being.GetName())

	user, ok := resp.Being.(*queryWithInterfaceNoFragmentsBeingUser)
	require.Truef(t, ok, "got %T, not User", resp.Being)
	assert.Equal(t, "1", user.Id)
	assert.Equal(t, "Yours Truly", user.Name)

	resp, err = queryWithInterfaceNoFragments(ctx, client, "3")
	require.NoError(t, err)

	// We should get the following response:
	//	me: User{Id: 1, Name: "Yours Truly"},
	//	being: Animal{Id: 3, Name: "Fido"},

	assert.Equal(t, "1", resp.Me.Id)
	assert.Equal(t, "Yours Truly", resp.Me.Name)

	assert.Equal(t, "Animal", resp.Being.GetTypename())
	assert.Equal(t, "3", resp.Being.GetId())
	assert.Equal(t, "Fido", resp.Being.GetName())

	animal, ok := resp.Being.(*queryWithInterfaceNoFragmentsBeingAnimal)
	require.Truef(t, ok, "got %T, not Animal", resp.Being)
	assert.Equal(t, "3", animal.Id)
	assert.Equal(t, "Fido", animal.Name)

	resp, err = queryWithInterfaceNoFragments(ctx, client, "4757233945723")
	require.NoError(t, err)

	// We should get the following response:
	//	me: User{Id: 1, Name: "Yours Truly"},
	//	being: null

	assert.Equal(t, "1", resp.Me.Id)
	assert.Equal(t, "Yours Truly", resp.Me.Name)

	assert.Nil(t, resp.Being)
}

func TestInterfaceListField(t *testing.T) {
	_ = `# @genqlient
	query queryWithInterfaceListField($ids: [ID!]!) {
		beings(ids: $ids) { id name }
	}`

	ctx := context.Background()
	server := server.RunServer()
	defer server.Close()
	client := graphql.NewClient(server.URL, http.DefaultClient)

	resp, err := queryWithInterfaceListField(ctx, client,
		[]string{"1", "3", "12847394823"})
	require.NoError(t, err)

	require.Len(t, resp.Beings, 3)

	// We should get the following three beings:
	//	User{Id: 1, Name: "Yours Truly"},
	//	Animal{Id: 3, Name: "Fido"},
	//	null

	// Check fields both via interface and via type-assertion:
	assert.Equal(t, "User", resp.Beings[0].GetTypename())
	assert.Equal(t, "1", resp.Beings[0].GetId())
	assert.Equal(t, "Yours Truly", resp.Beings[0].GetName())

	user, ok := resp.Beings[0].(*queryWithInterfaceListFieldBeingsUser)
	require.Truef(t, ok, "got %T, not User", resp.Beings[0])
	assert.Equal(t, "1", user.Id)
	assert.Equal(t, "Yours Truly", user.Name)

	assert.Equal(t, "Animal", resp.Beings[1].GetTypename())
	assert.Equal(t, "3", resp.Beings[1].GetId())
	assert.Equal(t, "Fido", resp.Beings[1].GetName())

	animal, ok := resp.Beings[1].(*queryWithInterfaceListFieldBeingsAnimal)
	require.Truef(t, ok, "got %T, not Animal", resp.Beings[1])
	assert.Equal(t, "3", animal.Id)
	assert.Equal(t, "Fido", animal.Name)

	assert.Nil(t, resp.Beings[2])
}

func TestInterfaceListPointerField(t *testing.T) {
	_ = `# @genqlient
	query queryWithInterfaceListPointerField($ids: [ID!]!) {
		# @genqlient(pointer: true)
		beings(ids: $ids) {
			__typename id name
		}
	}`

	ctx := context.Background()
	server := server.RunServer()
	defer server.Close()
	client := graphql.NewClient(server.URL, http.DefaultClient)

	resp, err := queryWithInterfaceListPointerField(ctx, client,
		[]string{"1", "3", "12847394823"})
	require.NoError(t, err)

	require.Len(t, resp.Beings, 3)

	user, ok := (*resp.Beings[0]).(*queryWithInterfaceListPointerFieldBeingsUser)
	require.Truef(t, ok, "got %T, not User", resp.Beings[0])
	assert.Equal(t, "1", user.Id)
	assert.Equal(t, "Yours Truly", user.Name)

	animal, ok := (*resp.Beings[1]).(*queryWithInterfaceListPointerFieldBeingsAnimal)
	require.Truef(t, ok, "got %T, not Animal", resp.Beings[1])
	assert.Equal(t, "3", animal.Id)
	assert.Equal(t, "Fido", animal.Name)

	assert.Nil(t, *resp.Beings[2])
}

func TestGeneratedCode(t *testing.T) {
	// TODO(benkraft): Check that gqlgen is up to date too.  In practice that's
	// less likely to be a problem, since it should only change if you update
	// the schema, likely too add something new, in which case you'll notice.
	RunGenerateTest(t, "internal/integration/genqlient.yaml")
}

//go:generate go run github.com/Khan/genqlient genqlient.yaml
