package generate

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/vektah/gqlparser/v2/ast"
)

var (
	parseDataDir   = "testdata/parsing"
	parseErrorsDir = "testdata/parsing-errors"
)

func sortQueries(queryDoc *ast.QueryDocument) {
	sort.Slice(queryDoc.Operations, func(i, j int) bool {
		return queryDoc.Operations[i].Name < queryDoc.Operations[j].Name
	})
	sort.Slice(queryDoc.Fragments, func(i, j int) bool {
		return queryDoc.Fragments[i].Name < queryDoc.Fragments[j].Name
	})
}

func getTestQueries(t *testing.T, ext string) *ast.QueryDocument {
	graphqlQueries, err := getQueries(
		parseDataDir, []string{filepath.Join(parseDataDir, "*."+ext)})
	if err != nil {
		t.Fatal(err)
	}

	// The different file-types may have the operations/fragments in a
	// different order.
	sortQueries(graphqlQueries)

	return graphqlQueries
}

// TestParse tests that query-extraction from different language source files
// produces equivalent results.  We do not test the results it produces (that's
// covered by TestGenerate), just that they are equivalent in different
// languages (since TestGenerate only uses .graphql as input).
func TestParse(t *testing.T) {
	extensions := []string{"go"}

	graphqlQueries := getTestQueries(t, "graphql")

	// check it's at least non-empty
	if len(graphqlQueries.Operations) == 0 || len(graphqlQueries.Fragments) == 0 {
		t.Fatalf("Didn't find any queries in *.graphql files")
	}

	sortQueries(graphqlQueries)

	for _, ext := range extensions {
		t.Run(ext, func(t *testing.T) {
			queries := getTestQueries(t, ext)

			got, want := ast.Dump(graphqlQueries), ast.Dump(queries)
			if got != want {
				// TODO: nice diffing
				t.Errorf("got:\n%v\nwant:\n%v\n", got, want)
			}
		})
	}
}

// TestParseErrors tests that query-extraction from different language source files
// produces appropriate errors if your query is invalid.
func TestParseErrors(t *testing.T) {
	extensions := []string{"graphql", "go"}

	for _, ext := range extensions {
		t.Run(ext, func(t *testing.T) {
			g, err := getQueries(
				parseErrorsDir,
				[]string{filepath.Join(parseErrorsDir, "*."+ext)})
			if err == nil {
				t.Errorf("expected error from getQueries(*.%v)", ext)
				t.Logf("%#v", g)
			}
		})
	}
}
