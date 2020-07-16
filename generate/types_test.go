package generate

import (
	"errors"
	"fmt"
	"go/format"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/vektah/gqlparser"
	"github.com/vektah/gqlparser/ast"
)

const dataDir = "testdata"

func readFile(t *testing.T, filename string, allowNotExist bool) string {
	t.Helper()
	data, err := ioutil.ReadFile(filepath.Join(dataDir, filename))
	if err != nil {
		if allowNotExist && errors.Is(err, os.ErrNotExist) {
			return ""
		}
		t.Fatal(err)
	}
	return string(data)
}

func gofmt(src string) (string, error) {
	src = strings.TrimSpace(src)
	formatted, err := format.Source([]byte(src))
	if err != nil {
		return src, fmt.Errorf("go parse error: %w", err)
	}
	return string(formatted), nil
}

func TestTypeForOperation(t *testing.T) {
	// This test uses the schema, queries, and expected-output in ./testdata.
	// The schema is in schema.graphql.  The queries are in TestName.graphql;
	// the test asserts that such queries, when run through the type-generator,
	// produce the types in TestName.go (the name of the overall response type
	// will be Response).
	//
	// Change update on the next line to true to update all the expected output
	// files to match current output.
	// TODO(benkraft): Make this a flag or something.
	update := false

	files, err := ioutil.ReadDir(dataDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		graphqlFilename := file.Name()
		if graphqlFilename == "schema.graphql" || !strings.HasSuffix(graphqlFilename, ".graphql") {
			continue
		}
		goFilename := graphqlFilename + ".go"

		t.Run(graphqlFilename, func(t *testing.T) {
			expectedGoCode, err := gofmt(readFile(t, goFilename, update))
			if err != nil {
				t.Fatal(err)
			}

			goCode, err := Generate(&Config{
				Schema:  filepath.Join("testdata", "schema.graphql"),
				Queries: filepath.Join("testdata", graphqlFilename),
				Package: "test",
			})
			if err != nil {
				t.Fatal(err)
			}

			if string(goCode) != expectedGoCode {
				t.Errorf("got:\n%v\nwant:\n%v\n", string(goCode), expectedGoCode)
				if update {
					t.Log("Updating testdata dir to match")
					err = ioutil.WriteFile(filepath.Join(dataDir, goFilename), goCode, 0644)
					if err != nil {
						t.Errorf("Unable to update testdata dir: %v", err)
					}
				}
			}
		})
	}

	if update {
		// This is an error to ensure we don't commit update := true
		t.Error("Updated testdata dir")
	}
}

// TODO(benkraft): Figure out how to do this with testdata-files
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
		`*UserQueryInput`,
		[]string{
			`type Role string
			const (
				StudentRole Role = "STUDENT"
				TeacherRole Role = "TEACHER"
			)`,
			`type UserQueryInput struct {
				Email    *string ` + "`json:\"email\"`" + `
				Name     *string ` + "`json:\"name\"`" + `
				Id       *string ` + "`json:\"id\"`" + `
				Role     *Role   ` + "`json:\"role\"`" + `
			}`,
		},
	}}

	schemaText := readFile(t, "schema.graphql", false)

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			sort.Strings(test.otherTypes) // To match generator.Types()
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

			g := newGenerator(&Config{Package: "test_package"}, schema)
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
