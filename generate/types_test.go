package generate

import (
	"fmt"
	"go/format"
	"strings"
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

var schemaText = `
	input UserQueryInput {
		email: String
		name: String
		id: ID
	}

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
`

func TestTypeForOperation(t *testing.T) {
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

			schema, graphqlError := gqlparser.LoadSchema(
				&ast.Source{Name: "test schema", Input: schemaText})
			if graphqlError != nil {
				t.Fatal(graphqlError)
			}

			queryDoc, graphqlListError := gqlparser.LoadQuery(schema, test.operation)
			if graphqlListError != nil {
				t.Fatal(graphqlListError)
			}

			if len(queryDoc.Operations) != 1 {
				t.Fatalf("got %v operations, want 1", len(queryDoc.Operations))
			}

			g := newGenerator("test_package", schema)
			name, err := g.getTypeForOperation(queryDoc.Operations[0])
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

func TestTypeForInputType(t *testing.T) {
	tests := []struct {
		name           string
		graphQLType    string
		expectedGoType string
		otherTypes     []string
	}{{
		`RequiredBuiltin`,
		`String!`,
		`string`,
		nil,
	}, {
		`ListOfBuiltin`,
		`[String]`,
		`[]*string`,
		nil,
	}, {
		`DefinedType`,
		`UserQueryInput`,
		`*userQueryInput`,
		[]string{`type userQueryInput struct {
			Email    *string ` + "`json:\"email\"`" + `
			Name     *string ` + "`json:\"name\"`" + `
			Id       *string ` + "`json:\"id\"`" + `
		}`},
	}}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			expectedGoCode := fmt.Sprintf(
				"type Input %s\n\n%s", test.expectedGoType,
				strings.Join(test.otherTypes, "\n\n"))
			expectedGoCode, err := gofmt(expectedGoCode)
			if err != nil {
				t.Fatal(err)
			}

			extraSchemaText := fmt.Sprintf(
				"extend type Query { testQuery(var: %s): User }", test.graphQLType)
			schema, graphqlError := gqlparser.LoadSchema(
				&ast.Source{Name: "test schema", Input: schemaText},
				&ast.Source{Name: "test schema extension", Input: extraSchemaText},
			)
			if graphqlError != nil {
				t.Fatal(graphqlError)
			}

			operation := fmt.Sprintf(
				"query($var: %s) { testQuery(var: $var) { id } }", test.graphQLType)
			queryDoc, graphqlListError := gqlparser.LoadQuery(schema, operation)
			if graphqlListError != nil {
				t.Fatal(graphqlListError)
			}

			g := newGenerator("test_package", schema)
			goType, err := g.getTypeForInputType(
				queryDoc.Operations[0].VariableDefinitions[0].Type)
			if err != nil {
				t.Error(err)
			}

			goCode := fmt.Sprintf("type Input %s\n\n%s", goType, g.Types())

			// gofmt before comparing.
			goCode, err = gofmt(goCode)
			if err != nil {
				t.Error(err)
			}

			if goCode != expectedGoCode {
				t.Errorf("got:\n%v\nwant:\n%v\n", goCode, expectedGoCode)
			}
		})
	}
}
