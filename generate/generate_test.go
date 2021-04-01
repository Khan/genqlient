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

func gofmt(filename, src string) (string, error) {
	src = strings.TrimSpace(src)
	formatted, err := format.Source([]byte(src))
	if err != nil {
		return src, fmt.Errorf("go parse error in %v: %w", filename, err)
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
		queriesFilename := graphqlFilename + ".json"

		t.Run(graphqlFilename, func(t *testing.T) {
			generated, err := Generate(&Config{
				Schema:           filepath.Join(dataDir, "schema.graphql"),
				Operations:       []string{filepath.Join(dataDir, graphqlFilename)},
				Package:          "test",
				Generated:        goFilename,
				ExportOperations: queriesFilename,
			})
			if err != nil {
				t.Fatal(err)
			}

			for filename, content := range generated {
				expectedContent := readFile(t, filename, update)
				if strings.HasSuffix(filename, ".go") {
					fmted, err := gofmt(filename, expectedContent)
					if err != nil {
						// Ignore gofmt errors if we are updating
						if !update {
							t.Fatal(err)
						}
					} else {
						expectedContent = fmted
					}
				}

				if string(content) != expectedContent {
					t.Errorf("mismatch in %v\ngot:\n%v\nwant:\n%v\n",
						filename, string(content), expectedContent)
					if update {
						t.Log("Updating testdata dir to match")
						err = ioutil.WriteFile(filepath.Join(dataDir, filename), content, 0o644)
						if err != nil {
							t.Errorf("Unable to update testdata dir: %v", err)
						}
					}
				}

				// TODO(benkraft): Also check that the code at least builds!
			}
		})
	}
}
