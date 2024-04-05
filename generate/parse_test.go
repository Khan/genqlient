package generate

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vektah/gqlparser/v2/ast"
)

var (
	parseDataDir       = "testdata/parsing"
	parseErrorsDir     = "testdata/parsing-errors"
	expandFilenamesDir = "testdata/expandFilenames"
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
// TODO: redo this as more standard snapshot tests?
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

			got, want := ast.Dump(graphqlQueries), removeComments(ast.Dump(queries))

			if got != want {
				// TODO: nice diffing
				t.Errorf("got:\n%v\nwant:\n%v\n", got, want)
			}
		})
	}
}

func removeComments(gotWithComments string) string {
	var gots []string
	for _, s := range strings.Split(gotWithComments, "\n") {
		if strings.Contains(s, `Comment:`) {
			continue
		}
		gots = append(gots, s)
	}
	got := strings.Join(gots, "\n")
	return got
}

func filepathJoinAll(a string, bs []string) []string {
	ret := make([]string, len(bs))
	for i, b := range bs {
		ret[i] = filepath.Join(a, b)
	}
	return ret
}

func TestExpandFilenames(t *testing.T) {
	tests := []struct {
		name  string
		globs []string
		files []string
		err   bool
	}{
		{"SingleFile", []string{"a/b/c"}, []string{"a/b/c"}, false},
		{"OneStar", []string{"a/*/c"}, []string{"a/b/c"}, false},
		{"StarExt", []string{"a/b/*"}, []string{"a/b/c", "a/b/c.d"}, false},
		{"TwoStar", []string{"**/c"}, []string{"a/b/c"}, false},
		{"TwoStarSuffix", []string{"**/*"}, []string{"a/b/c", "a/b/c.d"}, false},
		{"Repeated", []string{"a/b/c", "a/b/*"}, []string{"a/b/c", "a/b/c.d"}, false},
		{"Empty", []string{"bogus/*"}, nil, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files, err := expandFilenames(filepathJoinAll(expandFilenamesDir, test.globs))
			if test.err && err == nil {
				t.Errorf("got %v, wanted error", files)
			} else if !test.err && err != nil {
				t.Errorf("got error %v, wanted %v", err, test.files)
			} else {
				assert.ElementsMatch(t, filepathJoinAll(expandFilenamesDir, test.files), files)
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
