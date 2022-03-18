package generate

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Khan/genqlient/internal/testutil"
	"gopkg.in/yaml.v2"
)

const (
	dataDir   = "testdata/queries"
	errorsDir = "testdata/errors"
)

// buildGoFile returns an error if the given Go code is not valid.
//
// namePrefix is used for the temp-file, and is just for debugging.
func buildGoFile(namePrefix string, content []byte) error {
	// We need to put this within the current module, rather than in
	// /tmp, so that it can access internal/testutil.
	f, err := ioutil.TempFile("./testdata/tmp", namePrefix+"_*.go")
	if err != nil {
		return err
	}
	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()

	_, err = f.Write(content)
	if err != nil {
		return err
	}

	cmd := exec.Command("go", "build", f.Name())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("generated code does not compile: %w", err)
	}
	return nil
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
				Schema:           []string{filepath.Join(dataDir, "schema.graphql")},
				Operations:       []string{filepath.Join(dataDir, sourceFilename)},
				Package:          "test",
				Generated:        goFilename,
				ExportOperations: queriesFilename,
				ContextType:      "-",
				Bindings: map[string]*TypeBinding{
					"ID":       {Type: "github.com/Khan/genqlient/internal/testutil.ID"},
					"DateTime": {Type: "time.Time"},
					"Date": {
						Type:        "time.Time",
						Marshaler:   "github.com/Khan/genqlient/internal/testutil.MarshalDate",
						Unmarshaler: "github.com/Khan/genqlient/internal/testutil.UnmarshalDate",
					},
					"Junk":        {Type: "interface{}"},
					"ComplexJunk": {Type: "[]map[string]*[]*map[string]interface{}"},
					"Pokemon": {
						Type:              "github.com/Khan/genqlient/internal/testutil.Pokemon",
						ExpectExactFields: "{ species level }",
					},
					"PokemonInput": {Type: "github.com/Khan/genqlient/internal/testutil.Pokemon"},
				},
				AllowBrokenFeatures: true,
			})
			if err != nil {
				t.Fatal(err)
			}

			for filename, content := range generated {
				t.Run(filename, func(t *testing.T) {
					testutil.Cupaloy.SnapshotT(t, string(content))
				})
			}

			t.Run("Build", func(t *testing.T) {
				if testing.Short() {
					t.Skip("skipping build due to -short")
				}

				err := buildGoFile(sourceFilename, generated[goFilename])
				if err != nil {
					t.Error(err)
				}
			})
		})
	}
}

func defaultConfig(t *testing.T) *Config {
	// Parse the config that `genqlient --init` generates, to make sure that
	// works.
	var config Config
	b, err := ioutil.ReadFile("default_genqlient.yaml")
	if err != nil {
		t.Fatal(err)
	}

	err = yaml.UnmarshalStrict(b, &config)
	if err != nil {
		t.Fatal(err)
	}
	return &config
}

// TestGenerateWithConfig tests several configuration options that affect
// generated code but don't require particular query structures to test.
//
// It runs a simple query from TestGenerate with several different genqlient
// configurations.  It uses snapshots, just like TestGenerate.
func TestGenerateWithConfig(t *testing.T) {
	tests := []struct {
		name    string
		baseDir string  // relative to dataDir
		config  *Config // omits Schema and Operations, set below.
	}{
		{"DefaultConfig", "", defaultConfig(t)},
		{"Subpackage", "", &Config{
			Generated: "mypkg/myfile.go",
		}},
		{"SubpackageConfig", "mypkg", &Config{
			Generated: "myfile.go", // (relative to genqlient.yaml)
		}},
		{"PackageName", "", &Config{
			Generated: "myfile.go",
			Package:   "mypkg",
		}},
		{"ExportOperations", "", &Config{
			Generated:        "generated.go",
			ExportOperations: "operations.json",
		}},
		{"CustomContext", "", &Config{
			Generated:   "generated.go",
			ContextType: "github.com/Khan/genqlient/internal/testutil.MyContext",
		}},
		{"StructReferences", "", &Config{
			StructReferences: true,
			Generated:        "generated-structrefs.go",
		}},
		{"TypeAutoBindings", "", &Config{
			AutoBindings: StringList{
				"github.com/Khan/genqlient/internal/testutil",
			},
		}},
		{"NoContext", "", &Config{
			Generated:   "generated.go",
			ContextType: "-",
		}},
		{"ClientGetter", "", &Config{
			Generated:    "generated.go",
			ClientGetter: "github.com/Khan/genqlient/internal/testutil.GetClientFromContext",
		}},
		{"ClientGetterCustomContext", "", &Config{
			Generated:    "generated.go",
			ClientGetter: "github.com/Khan/genqlient/internal/testutil.GetClientFromMyContext",
			ContextType:  "github.com/Khan/genqlient/internal/testutil.MyContext",
		}},
		{"ClientGetterNoContext", "", &Config{
			Generated:    "generated.go",
			ClientGetter: "github.com/Khan/genqlient/internal/testutil.GetClientFromNowhere",
			ContextType:  "-",
		}},
	}

	sourceFilename := "SimpleQuery.graphql"

	for _, test := range tests {
		config := test.config
		baseDir := filepath.Join(dataDir, test.baseDir)
		t.Run(test.name, func(t *testing.T) {
			err := config.ValidateAndFillDefaults(baseDir)
			config.Schema = []string{filepath.Join(dataDir, "schema.graphql")}
			config.Operations = []string{filepath.Join(dataDir, sourceFilename)}
			if err != nil {
				t.Fatal(err)
			}
			generated, err := Generate(config)
			if err != nil {
				t.Fatal(err)
			}

			for filename, content := range generated {
				t.Run(filename, func(t *testing.T) {
					testutil.Cupaloy.SnapshotT(t, string(content))
				})
			}

			t.Run("Build", func(t *testing.T) {
				if testing.Short() {
					t.Skip("skipping build due to -short")
				}

				err := buildGoFile(sourceFilename,
					generated[config.Generated])
				if err != nil {
					t.Error(err)
				}
			})
		})
	}
}

// TestGenerate is a snapshot-based test of error text.
//
// For each .go or .graphql file in testdata/errors, and corresponding
// .schema.graphql file, it asserts that the given query returns an error, and
// that that error's string-text matches the snapshot.  The snapshotting is
// useful to ensure we don't accidentally make the text less readable, drop the
// line numbers, etc.  We include both .go and .graphql tests, to make sure the
// line numbers work in both cases.
func TestGenerateErrors(t *testing.T) {
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
		testFilename := strings.ReplaceAll(sourceFilename, ".", "/")

		t.Run(testFilename, func(t *testing.T) {
			_, err := Generate(&Config{
				Schema:      []string{filepath.Join(errorsDir, schemaFilename)},
				Operations:  []string{filepath.Join(errorsDir, sourceFilename)},
				Package:     "test",
				Generated:   os.DevNull,
				ContextType: "context.Context",
				Bindings: map[string]*TypeBinding{
					"ValidScalar":   {Type: "string"},
					"InvalidScalar": {Type: "bogus"},
					"Pokemon": {
						Type:              "github.com/Khan/genqlient/internal/testutil.Pokemon",
						ExpectExactFields: "{ species level }",
					},
				},
				AllowBrokenFeatures: true,
			})
			if err == nil {
				t.Fatal("expected an error")
			}

			testutil.Cupaloy.SnapshotT(t, err.Error())
		})
	}
}
