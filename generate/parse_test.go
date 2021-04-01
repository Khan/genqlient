package generate

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/vektah/gqlparser/v2/ast"
)

var parseDataDir = "testdata/parsing"

func sortQueries(queryDoc *ast.QueryDocument) {
	sort.Slice(queryDoc.Operations, func(i, j int) bool {
		return queryDoc.Operations[i].Name < queryDoc.Operations[j].Name
	})
	sort.Slice(queryDoc.Fragments, func(i, j int) bool {
		return queryDoc.Fragments[i].Name < queryDoc.Fragments[j].Name
	})
}

// TestParse tests that query-extraction from different language source files
// produces equivalent results.  We do not test the results it produces (that's
// covered by TestGenerate), just that they are equivalent in different
// languages (since TestGenerate only uses .graphql as input).
func TestParse(t *testing.T) {
	extensions := []string{"go"}

	graphqlQueries, err := getQueries([]string{filepath.Join(parseDataDir, "*.graphql")})
	if err != nil {
		t.Fatal(err)
	}

	// check it's at least non-empty
	if len(graphqlQueries.Operations) == 0 || len(graphqlQueries.Fragments) == 0 {
		t.Fatalf("Didn't find any queries in *.graphql files")
	}

	sortQueries(graphqlQueries)

	for _, ext := range extensions {
		t.Run(ext, func(t *testing.T) {
			queries, err := getQueries([]string{filepath.Join(parseDataDir, "*."+ext)})
			if err != nil {
				t.Fatal(err)
			}

			// The different file-types may have the operations/fragments in a
			// different order.
			sortQueries(queries)

			got, want := ast.Dump(graphqlQueries), ast.Dump(queries)
			if got != want {
				// TODO: nice diffing
				t.Errorf("got:\n%v\nwant:\n%v\n", got, want)
			}
		})
	}
}
