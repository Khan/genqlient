// Package integration contains genqlient's integration tests, which run
// against a real server (defined in internal/integration/server/server.go).
//
// These are especially important for cases where we generate nontrivial logic,
// such as JSON-unmarshaling.
package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Khan/genqlient/graphql"
	"github.com/Khan/genqlient/internal/integration/server"
)

func TestSimpleQuery(t *testing.T) {
	_ = `# @genqlient
	query simpleQuery { me { id name luckyNumber greatScalar } }`

	ctx := context.Background()
	server := server.RunServer()
	defer server.Close()
	clients := newRoundtripClients(t, server.URL)

	for _, client := range clients {
		resp, _, err := simpleQuery(ctx, client)
		require.NoError(t, err)

		assert.Equal(t, "1", resp.Me.Id)
		assert.Equal(t, "Yours Truly", resp.Me.Name)
		assert.Equal(t, 17, resp.Me.LuckyNumber)
	}
}

func TestMutation(t *testing.T) {
	_ = `# @genqlient
	mutation createUser($user: NewUser!) { createUser(input: $user) { id name } }`

	ctx := context.Background()
	server := server.RunServer()
	defer server.Close()
	postClient := newRoundtripClient(t, server.URL)
	getClient := newRoundtripGetClient(t, server.URL)

	resp, _, err := createUser(ctx, postClient, NewUser{Name: "Jack"})
	require.NoError(t, err)
	assert.Equal(t, "5", resp.CreateUser.Id)
	assert.Equal(t, "Jack", resp.CreateUser.Name)

	_, _, err = createUser(ctx, getClient, NewUser{Name: "Jill"})
	require.Errorf(t, err, "client does not support mutations")
}

type subscriptionResult struct {
	clientUnsubscribed  bool
	serverChannelClosed bool
}

func TestSubscription(t *testing.T) {
	_ = `# @genqlient
	subscription count { count }`

	ctx := context.Background()
	server := server.RunServer()
	defer server.Close()

	cases := []struct {
		name           string
		unsubThreshold time.Duration
		expected       subscriptionResult
	}{
		{
			name:           "server_closed_channel",
			unsubThreshold: 5 * time.Second,
			expected: subscriptionResult{
				clientUnsubscribed:  false,
				serverChannelClosed: true,
			},
		},
		{
			name:           "client_unsubscribed",
			unsubThreshold: 300 * time.Millisecond,
			expected: subscriptionResult{
				clientUnsubscribed:  true,
				serverChannelClosed: false,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wsClient := newRoundtripWebSocketClient(t, server.URL, nil)

			errChan, err := wsClient.Start(ctx)
			require.NoError(t, err)

			dataChan, subscriptionID, err := count(ctx, wsClient)
			require.NoError(t, err)
			defer wsClient.Close()

			var (
				counter = 0
				start   = time.Now()
				result  = subscriptionResult{}
			)

			for loop := true; loop; {
				select {
				case resp, more := <-dataChan:
					if !more {
						result.serverChannelClosed = true
						loop = false
						break
					}

					require.NotNil(t, resp.Data)
					assert.Equal(t, counter, resp.Data.Count)
					require.Nil(t, resp.Errors)

					if time.Since(start) > tc.unsubThreshold {
						err := wsClient.Unsubscribe(subscriptionID)
						require.NoError(t, err)
						result.clientUnsubscribed = true
						loop = false
					}

					counter++

				case err := <-errChan:
					require.NoError(t, err)

				case <-time.After(10 * time.Second):
					require.NoError(t, fmt.Errorf("subscription timed out"))
				}
			}

			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestSubscriptionConnectionParams(t *testing.T) {
	_ = `# @genqlient
	subscription countAuthorized { countAuthorized }`

	authKey := server.AuthKey

	ctx := context.Background()
	server := server.RunServer()
	defer server.Close()

	cases := []struct {
		connParams    map[string]interface{}
		name          string
		expectedError string
	}{
		{
			name: "authorized_user_gets_counter",
			connParams: map[string]interface{}{
				authKey: "authorized-user-token",
			},
		},
		{
			name:          "unauthorized_user_gets_error",
			expectedError: "input: countAuthorized unauthorized\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wsClient := newRoundtripWebSocketClient(t, server.URL, tc.connParams)

			errChan, err := wsClient.Start(ctx)
			require.NoError(t, err)

			dataChan, subscriptionID, err := countAuthorized(ctx, wsClient)
			require.NoError(t, err)
			defer wsClient.Close()

			var (
				counter = 0
				start   = time.Now()
			)

			for loop := true; loop; {
				select {
				case resp, more := <-dataChan:
					if !more {
						loop = false
						break
					}

					if tc.expectedError != "" {
						require.Error(t, resp.Errors)
						assert.Equal(t, tc.expectedError, resp.Errors.Error())
						continue
					}

					require.NotNil(t, resp.Data)
					assert.Equal(t, counter, resp.Data.CountAuthorized)
					require.Nil(t, resp.Errors)

					if time.Since(start) > 5*time.Second {
						err := wsClient.Unsubscribe(subscriptionID)
						require.NoError(t, err)
						loop = false
					}

					counter++

				case err := <-errChan:
					require.NoError(t, err)

				case <-time.After(10 * time.Second):
					require.NoError(t, fmt.Errorf("subscription timed out"))
				}
			}
		})
	}
}

func TestServerError(t *testing.T) {
	_ = `# @genqlient
	query failingQuery { fail me { id } }`

	ctx := context.Background()
	server := server.RunServer()
	defer server.Close()
	clients := newRoundtripClients(t, server.URL)

	for _, client := range clients {
		resp, _, err := failingQuery(ctx, client)
		// As long as we get some response back, we should still return a full
		// response -- and indeed in this case it should even have another field
		// (which didn't err) set.
		assert.Error(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "1", resp.Me.Id)
	}
}

func TestNetworkError(t *testing.T) {
	ctx := context.Background()
	clients := newRoundtripClients(t, "https://nothing.invalid/graphql")

	for _, client := range clients {
		resp, _, err := failingQuery(ctx, client)
		// As we guarantee in the README, even on network error you always get a
		// non-nil response; this is so you can write e.g.
		//	resp, err := failingQuery(ctx)
		//	return resp.Me.Id, err
		// without a bunch of extra ceremony.
		assert.Error(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, new(failingQueryResponse), resp)
	}
}

func TestVariables(t *testing.T) {
	_ = `# @genqlient
	query queryWithVariables($id: ID!) { user(id: $id) { id name luckyNumber } }`

	ctx := context.Background()
	server := server.RunServer()
	defer server.Close()
	// This doesn't roundtrip successfully because the zero user gets marshaled
	// as {"id": "", "name": "", ...}, not null.  There's really no way to do
	// this right in Go (without adding `pointer: true` just for this purpose),
	// and unmarshal(marshal(resp)) == resp should still hold, so we don't
	// worry about it.
	clients := []graphql.Client{
		graphql.NewClient(server.URL, http.DefaultClient),
		graphql.NewClientUsingGet(server.URL, http.DefaultClient),
	}

	for _, client := range clients {
		resp, _, err := queryWithVariables(ctx, client, "2")
		require.NoError(t, err)

		assert.Equal(t, "2", resp.User.Id)
		assert.Equal(t, "Raven", resp.User.Name)
		assert.Equal(t, -1, resp.User.LuckyNumber)

		resp, _, err = queryWithVariables(ctx, client, "374892379482379")
		require.NoError(t, err)

		assert.Zero(t, resp.User)
	}
}

func TestExtensions(t *testing.T) {
	_ = `# @genqlient
	query simpleQueryExt { me { id name luckyNumber } }`

	ctx := context.Background()
	server := server.RunServer()
	defer server.Close()
	clients := newRoundtripClients(t, server.URL)

	for _, client := range clients {
		_, extensions, err := simpleQueryExt(ctx, client)
		require.NoError(t, err)
		assert.NotNil(t, extensions)
		assert.Equal(t, extensions["foobar"], "test")
	}
}

func TestOmitempty(t *testing.T) {
	_ = `# @genqlient(omitempty: true)
	query queryWithOmitempty($id: ID) {
		user(id: $id) { id name luckyNumber }
	}`

	ctx := context.Background()
	server := server.RunServer()
	defer server.Close()
	clients := newRoundtripClients(t, server.URL)

	for _, client := range clients {
		resp, _, err := queryWithOmitempty(ctx, client, "2")
		require.NoError(t, err)

		assert.Equal(t, "2", resp.User.Id)
		assert.Equal(t, "Raven", resp.User.Name)
		assert.Equal(t, -1, resp.User.LuckyNumber)

		// should return default user, not the user with ID ""
		resp, _, err = queryWithOmitempty(ctx, client, "")
		require.NoError(t, err)

		assert.Equal(t, "1", resp.User.Id)
		assert.Equal(t, "Yours Truly", resp.User.Name)
		assert.Equal(t, 17, resp.User.LuckyNumber)
	}
}

func TestCustomMarshal(t *testing.T) {
	_ = `# @genqlient
	query queryWithCustomMarshal($date: Date!) {
		usersBornOn(date: $date) { id name birthdate }
	}`

	ctx := context.Background()
	server := server.RunServer()
	defer server.Close()
	clients := newRoundtripClients(t, server.URL)

	for _, client := range clients {
		resp, _, err := queryWithCustomMarshal(ctx, client,
			time.Date(2025, time.January, 1, 12, 34, 56, 789, time.UTC))
		require.NoError(t, err)

		assert.Len(t, resp.UsersBornOn, 1)
		user := resp.UsersBornOn[0]
		assert.Equal(t, "1", user.Id)
		assert.Equal(t, "Yours Truly", user.Name)
		assert.Equal(t,
			time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			user.Birthdate)

		resp, _, err = queryWithCustomMarshal(ctx, client,
			time.Date(2021, time.January, 1, 12, 34, 56, 789, time.UTC))
		require.NoError(t, err)
		assert.Len(t, resp.UsersBornOn, 0)
	}
}

func TestCustomMarshalSlice(t *testing.T) {
	_ = `# @genqlient
	query queryWithCustomMarshalSlice($dates: [Date!]!) {
		usersBornOnDates(dates: $dates) { id name birthdate }
	}`

	ctx := context.Background()
	server := server.RunServer()
	defer server.Close()
	clients := newRoundtripClients(t, server.URL)

	for _, client := range clients {
		resp, _, err := queryWithCustomMarshalSlice(ctx, client,
			[]time.Time{time.Date(2025, time.January, 1, 12, 34, 56, 789, time.UTC)})
		require.NoError(t, err)

		assert.Len(t, resp.UsersBornOnDates, 1)
		user := resp.UsersBornOnDates[0]
		assert.Equal(t, "1", user.Id)
		assert.Equal(t, "Yours Truly", user.Name)
		assert.Equal(t,
			time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			user.Birthdate)

		resp, _, err = queryWithCustomMarshalSlice(ctx, client,
			[]time.Time{time.Date(2021, time.January, 1, 12, 34, 56, 789, time.UTC)})
		require.NoError(t, err)
		assert.Len(t, resp.UsersBornOnDates, 0)
	}
}

func TestCustomMarshalOptional(t *testing.T) {
	_ = `# @genqlient
	query queryWithCustomMarshalOptional(
		# @genqlient(pointer: true)
		$date: Date,
		# @genqlient(pointer: true)
		$id: ID,
	) {
		userSearch(birthdate: $date, id: $id) { id name birthdate }
	}`

	ctx := context.Background()
	server := server.RunServer()
	defer server.Close()
	clients := newRoundtripClients(t, server.URL)

	for _, client := range clients {
		date := time.Date(2025, time.January, 1, 12, 34, 56, 789, time.UTC)
		resp, _, err := queryWithCustomMarshalOptional(ctx, client, &date, nil)
		require.NoError(t, err)

		assert.Len(t, resp.UserSearch, 1)
		user := resp.UserSearch[0]
		assert.Equal(t, "1", user.Id)
		assert.Equal(t, "Yours Truly", user.Name)
		assert.Equal(t,
			time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			user.Birthdate)

		id := "2"
		resp, _, err = queryWithCustomMarshalOptional(ctx, client, nil, &id)
		require.NoError(t, err)
		assert.Len(t, resp.UserSearch, 1)
		user = resp.UserSearch[0]
		assert.Equal(t, "2", user.Id)
		assert.Equal(t, "Raven", user.Name)
		assert.Zero(t, user.Birthdate)
	}
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
	clients := newRoundtripClients(t, server.URL)

	for _, client := range clients {
		resp, _, err := queryWithInterfaceNoFragments(ctx, client, "1")
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

		resp, _, err = queryWithInterfaceNoFragments(ctx, client, "3")
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

		resp, _, err = queryWithInterfaceNoFragments(ctx, client, "4757233945723")
		require.NoError(t, err)

		// We should get the following response:
		//	me: User{Id: 1, Name: "Yours Truly"},
		//	being: null

		assert.Equal(t, "1", resp.Me.Id)
		assert.Equal(t, "Yours Truly", resp.Me.Name)

		assert.Nil(t, resp.Being)
	}
}

func TestInterfaceListField(t *testing.T) {
	_ = `# @genqlient
	query queryWithInterfaceListField($ids: [ID!]!) {
		beings(ids: $ids) { id name }
	}`

	ctx := context.Background()
	server := server.RunServer()
	defer server.Close()
	clients := newRoundtripClients(t, server.URL)

	for _, client := range clients {
		resp, _, err := queryWithInterfaceListField(ctx, client,
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
	clients := newRoundtripClients(t, server.URL)

	for _, client := range clients {
		resp, _, err := queryWithInterfaceListPointerField(ctx, client,
			[]string{"1", "3", "12847394823"})
		require.NoError(t, err)

		require.Len(t, resp.Beings, 3)

		// Check fields both via interface and via type-assertion:
		assert.Equal(t, "User", (*resp.Beings[0]).GetTypename())
		assert.Equal(t, "1", (*resp.Beings[0]).GetId())
		assert.Equal(t, "Yours Truly", (*resp.Beings[0]).GetName())

		user, ok := (*resp.Beings[0]).(*queryWithInterfaceListPointerFieldBeingsUser)
		require.Truef(t, ok, "got %T, not User", *resp.Beings[0])
		assert.Equal(t, "1", user.Id)
		assert.Equal(t, "Yours Truly", user.Name)

		assert.Equal(t, "Animal", (*resp.Beings[1]).GetTypename())
		assert.Equal(t, "3", (*resp.Beings[1]).GetId())
		assert.Equal(t, "Fido", (*resp.Beings[1]).GetName())

		animal, ok := (*resp.Beings[1]).(*queryWithInterfaceListPointerFieldBeingsAnimal)
		require.Truef(t, ok, "got %T, not Animal", resp.Beings[1])
		assert.Equal(t, "3", animal.Id)
		assert.Equal(t, "Fido", animal.Name)

		assert.Nil(t, resp.Beings[2])
	}
}

func TestFragments(t *testing.T) {
	_ = `# @genqlient
	query queryWithFragments($ids: [ID!]!) {
		beings(ids: $ids) {
			__typename id
			... on Being { id name }
			... on Animal {
				id
				hair { hasHair }
				species
				owner {
					id
					... on Being { name }
					... on User { luckyNumber }
				}
			}
			... on Lucky { luckyNumber }
			... on User { hair { color } }
		}
	}`

	ctx := context.Background()
	server := server.RunServer()
	defer server.Close()
	clients := newRoundtripClients(t, server.URL)

	for _, client := range clients {
		resp, _, err := queryWithFragments(ctx, client, []string{"1", "3", "12847394823"})
		require.NoError(t, err)

		require.Len(t, resp.Beings, 3)

		// We should get the following three beings:
		//	User{Id: 1, Name: "Yours Truly"},
		//	Animal{Id: 3, Name: "Fido"},
		//	null

		// Check fields both via interface and via type-assertion when possible
		// User has, in total, the fields: __typename id name luckyNumber.
		assert.Equal(t, "User", resp.Beings[0].GetTypename())
		assert.Equal(t, "1", resp.Beings[0].GetId())
		assert.Equal(t, "Yours Truly", resp.Beings[0].GetName())
		// (hair and luckyNumber we need to cast for)

		user, ok := resp.Beings[0].(*queryWithFragmentsBeingsUser)
		require.Truef(t, ok, "got %T, not User", resp.Beings[0])
		assert.Equal(t, "1", user.Id)
		assert.Equal(t, "Yours Truly", user.Name)
		assert.Equal(t, "Black", user.Hair.Color)
		assert.Equal(t, 17, user.LuckyNumber)

		// Animal has, in total, the fields:
		//	__typename
		//	id
		//	species
		//	owner {
		//		id
		//		name
		//		... on User { luckyNumber }
		//	}
		assert.Equal(t, "Animal", resp.Beings[1].GetTypename())
		assert.Equal(t, "3", resp.Beings[1].GetId())
		// (hair, species, and owner.* we have to cast for)

		animal, ok := resp.Beings[1].(*queryWithFragmentsBeingsAnimal)
		require.Truef(t, ok, "got %T, not Animal", resp.Beings[1])
		assert.Equal(t, "3", animal.Id)
		assert.Equal(t, SpeciesDog, animal.Species)
		assert.True(t, animal.Hair.HasHair)

		assert.Equal(t, "1", animal.Owner.GetId())
		assert.Equal(t, "Yours Truly", animal.Owner.GetName())
		// (luckyNumber we have to cast for, again)

		owner, ok := animal.Owner.(*queryWithFragmentsBeingsAnimalOwnerUser)
		require.Truef(t, ok, "got %T, not User", animal.Owner)
		assert.Equal(t, "1", owner.Id)
		assert.Equal(t, "Yours Truly", owner.Name)
		assert.Equal(t, 17, owner.LuckyNumber)

		assert.Nil(t, resp.Beings[2])
	}
}

func TestNamedFragments(t *testing.T) {
	_ = `# @genqlient
	fragment AnimalFields on Animal {
		id
		hair { hasHair }
		owner { id ...UserFields ...LuckyFields }
	}

	fragment MoreUserFields on User {
		id
		hair { color }
	}

	fragment LuckyFields on Lucky {
		...MoreUserFields
		luckyNumber
	}
	
	fragment UserFields on User {
		id
		...LuckyFields
		...MoreUserFields
	}

	query queryWithNamedFragments($ids: [ID!]!) {
		beings(ids: $ids) {
			__typename id
			...AnimalFields
			...UserFields
		}
	}`

	ctx := context.Background()
	server := server.RunServer()
	defer server.Close()
	clients := newRoundtripClients(t, server.URL)

	for _, client := range clients {
		resp, _, err := queryWithNamedFragments(ctx, client, []string{"1", "3", "12847394823"})
		require.NoError(t, err)

		require.Len(t, resp.Beings, 3)

		// We should get the following three beings:
		//	User{Id: 1, Name: "Yours Truly"},
		//	Animal{Id: 3, Name: "Fido"},
		//	null

		// Check fields both via interface and via type-assertion when possible
		// User has, in total, the fields: __typename id luckyNumber.
		assert.Equal(t, "User", resp.Beings[0].GetTypename())
		assert.Equal(t, "1", resp.Beings[0].GetId())
		// (luckyNumber, hair we need to cast for)

		user, ok := resp.Beings[0].(*queryWithNamedFragmentsBeingsUser)
		require.Truef(t, ok, "got %T, not User", resp.Beings[0])
		assert.Equal(t, "1", user.Id)
		assert.Equal(t, "1", user.UserFields.Id)
		assert.Equal(t, "1", user.UserFields.MoreUserFields.Id)
		assert.Equal(t, "1", user.UserFields.LuckyFieldsUser.MoreUserFields.Id)
		// on UserFields, but we should be able to access directly via embedding:
		assert.Equal(t, 17, user.LuckyNumber)
		assert.Equal(t, "Black", user.Hair.Color)
		assert.Equal(t, "Black", user.UserFields.MoreUserFields.Hair.Color)
		assert.Equal(t, "Black", user.UserFields.LuckyFieldsUser.MoreUserFields.Hair.Color)

		// Animal has, in total, the fields:
		//	__typename
		//	id
		//	hair { hasHair }
		//	owner { id luckyNumber }
		assert.Equal(t, "Animal", resp.Beings[1].GetTypename())
		assert.Equal(t, "3", resp.Beings[1].GetId())
		// (hair.* and owner.* we have to cast for)

		animal, ok := resp.Beings[1].(*queryWithNamedFragmentsBeingsAnimal)
		require.Truef(t, ok, "got %T, not Animal", resp.Beings[1])
		// Check that we filled in *both* ID fields:
		assert.Equal(t, "3", animal.Id)
		assert.Equal(t, "3", animal.AnimalFields.Id)
		// on AnimalFields:
		assert.True(t, animal.Hair.HasHair)
		assert.Equal(t, "1", animal.Owner.GetId())
		// (luckyNumber we have to cast for, again)

		owner, ok := animal.Owner.(*AnimalFieldsOwnerUser)
		require.Truef(t, ok, "got %T, not User", animal.Owner)
		// Check that we filled in *both* ID fields:
		assert.Equal(t, "1", owner.Id)
		assert.Equal(t, "1", owner.UserFields.Id)
		assert.Equal(t, "1", owner.UserFields.MoreUserFields.Id)
		assert.Equal(t, "1", owner.UserFields.LuckyFieldsUser.MoreUserFields.Id)
		// on UserFields:
		assert.Equal(t, 17, owner.LuckyNumber)
		assert.Equal(t, "Black", owner.UserFields.MoreUserFields.Hair.Color)
		assert.Equal(t, "Black", owner.UserFields.LuckyFieldsUser.MoreUserFields.Hair.Color)

		// Lucky-based fields we can also get by casting to the fragment-interface.
		luckyOwner, ok := animal.Owner.(LuckyFields)
		require.Truef(t, ok, "got %T, not Lucky", animal.Owner)
		assert.Equal(t, 17, luckyOwner.GetLuckyNumber())

		assert.Nil(t, resp.Beings[2])
	}
}

func TestFlatten(t *testing.T) {
	_ = `# @genqlient
	# @genqlient(flatten: true)
	fragment BeingFields on Being {
		...InnerBeingFields
	}

	fragment InnerBeingFields on Being {
		id
		name
		... on User {
			# @genqlient(flatten: true)
			friends {
				...FriendsFields
			}
		}
	}

	fragment FriendsFields on User {
		id
		name
	}

	# @genqlient(flatten: true)
	fragment FlattenedUserFields on User {
		...FlattenedLuckyFields
	}

	# @genqlient(flatten: true)
	fragment FlattenedLuckyFields on Lucky {
		...InnerLuckyFields
	}

	fragment InnerLuckyFields on Lucky {
		luckyNumber
	}
	
	fragment QueryFragment on Query {
		beings(ids: $ids) {
			__typename id
			...FlattenedUserFields
			... on Animal {
				# @genqlient(flatten: true)
				owner {
					...BeingFields
				}
			}
		}
	}

	# @genqlient(flatten: true)
	query queryWithFlatten(
		$ids: [ID!]!,
	) {
		...QueryFragment
	}`

	ctx := context.Background()
	server := server.RunServer()
	defer server.Close()
	clients := newRoundtripClients(t, server.URL)

	for _, client := range clients {
		resp, _, err := queryWithFlatten(ctx, client, []string{"1", "3", "12847394823"})
		require.NoError(t, err)

		require.Len(t, resp.Beings, 3)

		// We should get the following three beings:
		//	User{Id: 1, Name: "Yours Truly"},
		//	Animal{Id: 3, Name: "Fido"},
		//	null

		// Check fields both via interface and via type-assertion when possible
		// User has, in total, the fields: __typename id luckyNumber.
		assert.Equal(t, "User", resp.Beings[0].GetTypename())
		assert.Equal(t, "1", resp.Beings[0].GetId())
		// (luckyNumber we need to cast for)

		user, ok := resp.Beings[0].(*QueryFragmentBeingsUser)
		require.Truef(t, ok, "got %T, not User", resp.Beings[0])
		assert.Equal(t, "1", user.Id)
		assert.Equal(t, 17, user.InnerLuckyFieldsUser.LuckyNumber)

		// Animal has, in total, the fields:
		//	__typename
		//	id
		//	owner { id name ... on User { friends { id name } } }
		assert.Equal(t, "Animal", resp.Beings[1].GetTypename())
		assert.Equal(t, "3", resp.Beings[1].GetId())
		// (owner.* we have to cast for)

		animal, ok := resp.Beings[1].(*QueryFragmentBeingsAnimal)
		require.Truef(t, ok, "got %T, not Animal", resp.Beings[1])
		assert.Equal(t, "3", animal.Id)
		// on AnimalFields:
		assert.Equal(t, "1", animal.Owner.GetId())
		assert.Equal(t, "Yours Truly", animal.Owner.GetName())
		// (friends.* we have to cast for, again)

		owner, ok := animal.Owner.(*InnerBeingFieldsUser)
		require.Truef(t, ok, "got %T, not User", animal.Owner)
		assert.Equal(t, "1", owner.Id)
		assert.Equal(t, "Yours Truly", owner.Name)
		assert.Len(t, owner.Friends, 1)
		assert.Equal(t, "2", owner.Friends[0].Id)
		assert.Equal(t, "Raven", owner.Friends[0].Name)

		assert.Nil(t, resp.Beings[2])
	}
}

func TestGeneratedCode(t *testing.T) {
	// TODO(benkraft): Check that gqlgen is up to date too.  In practice that's
	// less likely to be a problem, since it should only change if you update
	// the schema, likely too add something new, in which case you'll notice.
	RunGenerateTest(t, "internal/integration/genqlient.yaml")
}

//go:generate go run github.com/Khan/genqlient genqlient.yaml
