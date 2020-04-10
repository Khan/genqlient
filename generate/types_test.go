package generate

import (
	"go/format"
	"testing"

	"github.com/vektah/gqlparser"
	"github.com/vektah/gqlparser/ast"
)

func gofmt(src string) (string, error) {
	formatted, err := format.Source([]byte(src))
	if err != nil {
		return src, err
	}
	return string(formatted), nil
}

func TestTypeForOperation(t *testing.T) {
	schema, err := gqlparser.LoadSchema(&ast.Source{Name: "test schema", Input: `
		type AuthMethod {
			provider: String
			email: String
		}

		type User {
			id: ID!
			name: String
			emails: [String!]!
			emailsOrNull: [String!]
			emailsWithNulls: [String]!
			emailsWithNullsOrNull: [String]
			authMethods: [AuthMethod!]!
		}

		type Query {
			user: User
		}
	`})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name           string
		operation      string
		expectedGoType string
	}{{
		"SimpleQuery",
		`{ user { id } }`,
		`type Response struct{
			User *struct {
				Id string ` + "`json:\"id\"`" + `
			} ` + "`json:\"user\"`" + `
		}`,
	}, {
		"QueryWithAlias",
		`{ User: user { ID: id } }`,
		`type Response struct{
			User *struct {
				ID string
			}
		}`,
		// Here on out, we use aliases, just because aliases are a lot less
		// annoying to write in Go strings than Go struct tags.
	}, {
		"QueryWithSlices",
		`{
			User: user {
				Emails: emails
				EmailsOrNull: emailsOrNull
				EmailsWithNulls: emailsWithNulls
				EmailsWithNullsOrNull: emailsWithNullsOrNull
			}
		}`,
		`type Response struct{
			User *struct {
				Emails                []string
				EmailsOrNull          []string
				EmailsWithNulls       []*string
				EmailsWithNullsOrNull []*string
			}
		}`,
	}, {
		"QueryWithStructs",
		`{
			User: user {
				AuthMethods: authMethods {
					Provider: provider
					Email: email
				}
			}
		}`,
		`type Response struct{
			User *struct {
				AuthMethods []struct {
					Provider *string
					Email    *string
				}
			}
		}`,
	}}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			expectedGoType, err := gofmt(test.expectedGoType)
			if err != nil {
				t.Fatal(err)
			}

			queryDoc, graphqlError := gqlparser.LoadQuery(schema, test.operation)
			if graphqlError != nil {
				t.Fatal(graphqlError)
			}

			if len(queryDoc.Operations) != 1 {
				t.Fatalf("got %v operations, want 1", len(queryDoc.Operations))
			}

			g := newGenerator("test_package", schema)
			name, err := g.addTypeForOperation(queryDoc.Operations[0])
			if err != nil {
				t.Error(err)
			}

			// gofmt before comparing.
			goType, err := gofmt(g.typeMap[name])
			if err != nil {
				t.Error(err)
			}

			if goType != expectedGoType {
				t.Errorf("got:\n%v\nwant:\n%v\n", goType, expectedGoType)
			}
		})
	}
}
