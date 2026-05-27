// Package omitimpls contains integration tests for the
// omit_unreferenced_implementations config option, run against the shared
// gqlgen server in ../server.
package omitimpls

//go:generate go run github.com/Khan/genqlient genqlient.yaml

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Khan/genqlient/graphql"
	"github.com/Khan/genqlient/internal/integration/server"
)

func TestCatchAllForUnreferencedImplementation(t *testing.T) {
	_ = `# @genqlient
	query queryOmitImpls($id: ID!) {
		being(id: $id) {
			id
			name
			... on User {
				luckyNumber
			}
		}
	}`

	ctx := context.Background()
	srv := server.RunServer()
	defer srv.Close()
	client := graphql.NewClient(srv.URL, http.DefaultClient)

	// id=1 → server returns a User, which is the only fragment-referenced
	// concrete implementation: we should get the typed-impl path with a
	// non-zero LuckyNumber.
	resp, err := queryOmitImpls(ctx, client, "1")
	require.NoError(t, err)

	assert.Equal(t, "User", resp.Being.GetTypename())
	assert.Equal(t, "1", resp.Being.GetId())
	assert.Equal(t, "Yours Truly", resp.Being.GetName())

	user, ok := resp.Being.(*queryOmitImplsBeingUser)
	require.Truef(t, ok, "got %T, not User", resp.Being)
	assert.Equal(t, 17, user.LuckyNumber)

	// id=3 → server returns an Animal. Animal is NOT fragment-referenced,
	// so it must fall through to the generated catch-all struct, which
	// still exposes the interface's shared fields via getters.
	resp, err = queryOmitImpls(ctx, client, "3")
	require.NoError(t, err)

	assert.Equal(t, "Animal", resp.Being.GetTypename())
	assert.Equal(t, "3", resp.Being.GetId())
	assert.Equal(t, "Fido", resp.Being.GetName())

	other, ok := resp.Being.(*queryOmitImplsBeingGenqlientOther)
	require.Truef(t, ok, "got %T, expected catch-all queryOmitImplsBeingGenqlientOther", resp.Being)
	assert.Equal(t, "Animal", other.Typename)
	assert.Equal(t, "3", other.Id)
	assert.Equal(t, "Fido", other.Name)

	// id=missing → server returns null. Both branches return a nil
	// interface; the catch-all must not be instantiated for null.
	resp, err = queryOmitImpls(ctx, client, "9999999")
	require.NoError(t, err)
	assert.Nil(t, resp.Being)
}
