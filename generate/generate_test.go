package generate

import (
	"errors"
	"fmt"
	"go/format"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const dataDir = "testdata/queries"

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

// TestGenerate is a snapshot-based test of code-generation.
//
// This file just has the test runner; the actual data is all in
// testdata/queries.  Specifically, the schema used for all the queries is in
// schema.graphql; the queries themselves are in TestName.graphql.  The test
// asserts that running genqlient on that query produces the generated code in
// the snapshot-file TestName.graphql.go.
//
// To update the snapshots (if the code-generator has changed), run the test
// with `UPDATE_SNAPSHOTS=1`; it will fail the tests and print any diffs, but
// update the snapshots.  Make sure to check that the output is sensible; the
// snapshots don't even get compiled!
func TestGenerate(t *testing.T) {
	update := (os.Getenv("UPDATE_SNAPSHOTS") == "1")

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
				Schema:  filepath.Join(dataDir, "schema.graphql"),
				Queries: []string{filepath.Join(dataDir, graphqlFilename)},
				Package: "test",
			})
			if err != nil {
				t.Fatal(err)
			}

			if string(goCode) != expectedGoCode {
				t.Errorf("got:\n%v\nwant:\n%v\n", string(goCode), expectedGoCode)
				if update {
					t.Log("Updating testdata dir to match")
					err = ioutil.WriteFile(filepath.Join(dataDir, goFilename), goCode, 0o644)
					if err != nil {
						t.Errorf("Unable to update testdata dir: %v", err)
					}
				}
			}

			// TODO(benkraft): Also check that the code at least builds!
		})
	}
}
