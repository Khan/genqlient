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
		being(id: $id) { __typename id name }
		me { id name }
	}`

	ctx := context.Background()
	server := server.RunServer()
	defer server.Close()
	client := graphql.NewClient(server.URL, http.DefaultClient)

	resp, err := queryWithInterfaceNoFragments(ctx, client, "1")
	require.NoError(t, err)

	assert.Equal(t, "1", resp.Me.Id)
	assert.Equal(t, "Yours Truly", resp.Me.Name)

	user, ok := resp.Being.(*queryWithInterfaceNoFragmentsBeingUser)
	require.Truef(t, ok, "got %T, not User", resp.Being)
	assert.Equal(t, "1", user.Id)
	assert.Equal(t, "Yours Truly", user.Name)

	resp, err = queryWithInterfaceNoFragments(ctx, client, "3")
	require.NoError(t, err)

	assert.Equal(t, "1", resp.Me.Id)
	assert.Equal(t, "Yours Truly", resp.Me.Name)

	animal, ok := resp.Being.(*queryWithInterfaceNoFragmentsBeingAnimal)
	require.Truef(t, ok, "got %T, not Animal", resp.Being)
	assert.Equal(t, "3", animal.Id)
	assert.Equal(t, "Fido", animal.Name)

	resp, err = queryWithInterfaceNoFragments(ctx, client, "4757233945723")
	require.NoError(t, err)

	assert.Equal(t, "1", resp.Me.Id)
	assert.Equal(t, "Yours Truly", resp.Me.Name)

	assert.Nil(t, resp.Being)
}

func TestGeneratedCode(t *testing.T) {
	// TODO(benkraft): Check that gqlgen is up to date too.  In practice that's
	// less likely to be a problem, since it should only change if you update
	// the schema, likely too add something new, in which case you'll notice.
	RunGenerateTest(t, "internal/integration/genqlient.yaml")
}

//go:generate go run github.com/Khan/genqlient genqlient.yaml
