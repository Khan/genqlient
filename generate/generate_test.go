package generate

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Khan/genqlient/internal/testutil"
)

const (
	dataDir   = "testdata/queries"
	errorsDir = "testdata/errors"
)

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
	// we can test parts of features even if they're not done yet!
	allowBrokenFeatures = true

	files, err := ioutil.ReadDir(dataDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		sourceFilename := file.Name()
		if sourceFilename == "schema.graphql" || !strings.HasSuffix(sourceFilename, ".graphql") {
			continue
		}
		goFilename := sourceFilename + ".go"
		queriesFilename := sourceFilename + ".json"

		t.Run(sourceFilename, func(t *testing.T) {
			generated, err := Generate(&Config{
				Schema:           filepath.Join(dataDir, "schema.graphql"),
				Operations:       []string{filepath.Join(dataDir, sourceFilename)},
				Package:          "test",
				Generated:        goFilename,
				ExportOperations: queriesFilename,
				Scalars: map[string]string{
					"ID":          "github.com/Khan/genqlient/internal/testutil.ID",
					"DateTime":    "time.Time",
					"Junk":        "interface{}",
					"ComplexJunk": "[]map[string]*[]*map[string]interface{}",
				},
			})
			if err != nil {
				t.Fatal(err)
			}

			for filename, content := range generated {
				t.Run(filename, func(t *testing.T) {
					testutil.Cupaloy.SnapshotT(t, string(content))
				})
				// TODO(benkraft): Also check that the code at least builds!
			}
		})
	}
}

func TestGenerateErrors(t *testing.T) {
	// we can test parts of features even if they're not done yet!
	allowBrokenFeatures = true

	files, err := ioutil.ReadDir(errorsDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		sourceFilename := file.Name()
		if !strings.HasSuffix(sourceFilename, ".graphql") &&
			!strings.HasSuffix(sourceFilename, ".go") ||
			strings.HasSuffix(sourceFilename, ".schema.graphql") {
			continue
		}

		baseFilename := strings.TrimSuffix(sourceFilename, filepath.Ext(sourceFilename))
		schemaFilename := baseFilename + ".schema.graphql"

		t.Run(sourceFilename, func(t *testing.T) {
			_, err := Generate(&Config{
				Schema:     filepath.Join(errorsDir, schemaFilename),
				Operations: []string{filepath.Join(errorsDir, sourceFilename)},
				Package:    "test",
				Generated:  os.DevNull,
				Scalars: map[string]string{
					"ValidScalar":   "string",
					"InvalidScalar": "bogus",
				},
			})
			if err == nil {
				t.Fatal("expected an error")
			}

			testutil.Cupaloy.SnapshotT(t, err.Error())
		})
	}
}
