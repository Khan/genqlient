package generate

import (
	"errors"
	"fmt"
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
	f, err := os.CreateTemp("./testdata/tmp", namePrefix+"_*.go")
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
	files, err := os.ReadDir(dataDir)
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

func getDefaultConfig(t *testing.T) *Config {
	// Parse the config that `genqlient --init` generates, to make sure that
	// works.
	var config Config
	b, err := os.ReadFile("default_genqlient.yaml")
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
		name       string
		baseDir    string   // relative to dataDir
		operations []string // overrides the default set below
		config     *Config  // omits Schema and Operations, set below.
	}{
		{"DefaultConfig", "", nil, getDefaultConfig(t)},
		{"Subpackage", "", nil, &Config{
			Generated: "mypkg/myfile.go",
		}},
		{"SubpackageConfig", "mypkg", nil, &Config{
			Generated: "myfile.go", // (relative to genqlient.yaml)
		}},
		{"PackageName", "", nil, &Config{
			Generated: "myfile.go",
			Package:   "mypkg",
		}},
		{"ExportOperations", "", nil, &Config{
			ExportOperations: "operations.json",
		}},
		{"CustomContext", "", nil, &Config{
			ContextType: "github.com/Khan/genqlient/internal/testutil.MyContext",
		}},
		{"CustomContextWithAlias", "", nil, &Config{
			ContextType: "github.com/Khan/genqlient/internal/testutil/junk---fun.name.MyContext",
		}},
		{"StructReferences", "", []string{"InputObject.graphql", "QueryWithStructs.graphql"}, &Config{
			StructReferences: true,
			Bindings: map[string]*TypeBinding{
				"Date": {
					Type:        "time.Time",
					Marshaler:   "github.com/Khan/genqlient/internal/testutil.MarshalDate",
					Unmarshaler: "github.com/Khan/genqlient/internal/testutil.UnmarshalDate",
				},
			},
		}},
		{"StructReferencesAndOptionalPointer", "", []string{"InputObject.graphql", "QueryWithStructs.graphql"}, &Config{
			StructReferences: true,
			Optional:         "pointer",
			Bindings: map[string]*TypeBinding{
				"Date": {
					Type:        "time.Time",
					Marshaler:   "github.com/Khan/genqlient/internal/testutil.MarshalDate",
					Unmarshaler: "github.com/Khan/genqlient/internal/testutil.UnmarshalDate",
				},
			},
		}},
		{"PackageBindings", "", nil, &Config{
			PackageBindings: []*PackageBinding{
				{Package: "github.com/Khan/genqlient/internal/testutil"},
			},
		}},
		{"NoContext", "", nil, &Config{
			ContextType: "-",
		}},
		{"ClientGetter", "", nil, &Config{
			ClientGetter: "github.com/Khan/genqlient/internal/testutil.GetClientFromContext",
		}},
		{"ClientGetterCustomContext", "", nil, &Config{
			ClientGetter: "github.com/Khan/genqlient/internal/testutil.GetClientFromMyContext",
			ContextType:  "github.com/Khan/genqlient/internal/testutil.MyContext",
		}},
		{"ClientGetterNoContext", "", nil, &Config{
			ClientGetter: "github.com/Khan/genqlient/internal/testutil.GetClientFromNowhere",
			ContextType:  "-",
		}},
		{"Extensions", "", nil, &Config{
			Extensions: true,
		}},
		{"OptionalValue", "", []string{"ListInput.graphql", "QueryWithSlices.graphql"}, &Config{
			Optional: "value",
		}},
		{"OptionalPointer", "", []string{"ListInput.graphql", "QueryWithSlices.graphql"}, &Config{
			Optional: "pointer",
		}},
		{"OptionalGeneric", "", []string{"ListInput.graphql", "QueryWithSlices.graphql"}, &Config{
			Optional:            "generic",
			OptionalGenericType: "github.com/Khan/genqlient/internal/testutil.Option",
		}},
		{"EnumRawCasingAll", "", []string{"QueryWithEnums.graphql"}, &Config{
			Casing: Casing{
				AllEnums: CasingRaw,
			},
		}},
		{"EnumRawCasingSpecific", "", []string{"QueryWithEnums.graphql"}, &Config{
			Casing: Casing{
				Enums: map[string]CasingAlgorithm{"Role": CasingRaw},
			},
		}},
	}

	sourceFilename := "SimpleQuery.graphql"

	for _, test := range tests {
		config := test.config
		baseDir := filepath.Join(dataDir, test.baseDir)
		t.Run(test.name, func(t *testing.T) {
			err := config.ValidateAndFillDefaults(baseDir)
			config.Schema = []string{filepath.Join(dataDir, "schema.graphql")}
			if test.operations == nil {
				config.Operations = []string{filepath.Join(dataDir, sourceFilename)}
			} else {
				config.Operations = make([]string, len(test.operations))
				for i := range test.operations {
					config.Operations[i] = filepath.Join(dataDir, test.operations[i])
				}
			}
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

// TestGenerateErrors is a snapshot-based test of error text.
//
// For each .go or .graphql file in testdata/errors, it asserts that the given
// query returns an error, and that that error's string-text matches the
// snapshot.  The snapshotting is useful to ensure we don't accidentally make
// the text less readable, drop the line numbers, etc.  We include both .go and
// .graphql tests for some of the test cases, to make sure the line numbers
// work in both cases.  Tests may include a .schema.graphql file of their own,
// or use the shared schema.graphql in the same directory for convenience.
func TestGenerateErrors(t *testing.T) {
	files, err := os.ReadDir(errorsDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		sourceFilename := file.Name()
		if !strings.HasSuffix(sourceFilename, ".graphql") &&
			!strings.HasSuffix(sourceFilename, ".go") ||
			strings.HasSuffix(sourceFilename, ".schema.graphql") ||
			sourceFilename == "schema.graphql" {
			continue
		}

		baseFilename := strings.TrimSuffix(sourceFilename, filepath.Ext(sourceFilename))
		testFilename := strings.ReplaceAll(sourceFilename, ".", "/")

		// Schema is either <base>.schema.graphql, or <dir>/schema.graphql if
		// that doesn't exist.
		schemaFilename := baseFilename + ".schema.graphql"
		if _, err := os.Stat(filepath.Join(errorsDir, schemaFilename)); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				schemaFilename = "schema.graphql"
			} else {
				t.Fatal(err)
			}
		}

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
