package generate

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSelfRecursiveInput(t *testing.T) {
	const schema = `
input Input {
  next: Input
  children: [Input]
  matrix: [[Input]]
  value: Int
}
type Query { test(input: Input): Boolean! }
`
	const query = `query Recursive(
  $input: Input
) { test(input: $input) }`
	tests := []struct {
		name   string
		query  string
		config Config
		fields map[string]string
		err    string
	}{
		{
			name:   "Default",
			fields: map[string]string{"Next": "*Input", "Children": "[]Input", "Matrix": "[][]Input", "Value": "int"},
		},
		{
			name:   "ExplicitPointer",
			query:  "# @genqlient(for: \"Input.next\", pointer: true)\n" + query,
			fields: map[string]string{"Next": "*Input", "Children": "[]Input"},
		},
		{
			name:  "ExplicitValue",
			query: "# @genqlient(for: \"Input.next\", pointer: false)\n" + query,
			err:   "pointer: false is not allowed on a self-recursive input field: Input.next",
		},
		{
			name:  "OperationExplicitValue",
			query: "# @genqlient(pointer: false)\n" + query,
			err:   "pointer: false is not allowed on a self-recursive input field: Input.next",
		},
		{
			name: "OptionalValue", config: Config{Optional: "value"},
			fields: map[string]string{"Next": "*Input", "Children": "[]Input", "Value": "int"},
		},
		{
			name: "OptionalPointer", config: Config{Optional: "pointer"},
			fields: map[string]string{"Next": "*Input", "Children": "[]*Input", "Matrix": "[][]*Input", "Value": "*int"},
		},
		{
			name: "OptionalPointerOmitEmpty", config: Config{Optional: "pointer_omitempty"},
			fields: map[string]string{"Next": "*Input", "Children": "[]*Input", "Value": "*int"},
		},
		{
			name: "StructReferences", config: Config{StructReferences: true},
			fields: map[string]string{"Next": "*Input", "Children": "[]*Input", "Value": "int"},
		},
		{
			name: "RenamedAndReused",
			query: `query Recursive(
  # @genqlient(typename: "RenamedInput")
  $input: Input
) { test(input: $input) }
query Reused($input: Input) { test(input: $input) }`,
			fields: map[string]string{"Next": "*Input", "RenamedInput.Next": "Input", "Children": "[]Input"},
		},
		{
			name:   "FieldTypeName",
			query:  "# @genqlient(for: \"Input.next\", typename: \"RenamedInput\")\n" + query,
			fields: map[string]string{"Next": "RenamedInput", "RenamedInput.Next": "RenamedInputInput", "RenamedInputInput.Next": "*RenamedInputInput", "Children": "[]Input"},
		},
		{
			name:   "ListValueOverride",
			query:  "# @genqlient(for: \"Input.children\", pointer: false)\n# @genqlient(for: \"Input.matrix\", pointer: false)\n" + query,
			config: Config{Optional: "pointer"},
			fields: map[string]string{"Next": "*Input", "Children": "[]Input", "Matrix": "[][]Input"},
		},
		{
			name:   "LocalBinding",
			query:  "# @genqlient(for: \"Input.next\", bind: \"map[string]interface{}\", pointer: false)\n" + query,
			fields: map[string]string{"Next": "map[string]interface{}", "Children": "[]Input"},
		},
		{
			name:   "GlobalBinding",
			config: Config{Bindings: map[string]*TypeBinding{"Input": {Type: "map[string]interface{}"}}},
			fields: map[string]string{"__RecursiveInput.Input": "map[string]interface{}"},
		},
		{
			name: "GlobalBindingOverride",
			query: `# @genqlient(for: "Input.next", bind: "-")
query Recursive(
  # @genqlient(bind: "-")
  $input: Input
) { test(input: $input) }`,
			config: Config{Bindings: map[string]*TypeBinding{"Input": {Type: "map[string]interface{}"}}},
			fields: map[string]string{"Next": "*Input", "Children": "[]map[string]interface{}"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			operation := test.query
			if operation == "" {
				operation = query
			}
			generated, err := generateInputForTest(t, schema, operation, &test.config)
			if test.err != "" {
				require.ErrorContains(t, err, test.err)
				return
			}
			require.NoError(t, err)
			for name, typ := range test.fields {
				structName, fieldName, qualified := strings.Cut(name, ".")
				if !qualified {
					structName, fieldName = "Input", name
				}
				pattern := `(?m)^type ` + regexp.QuoteMeta(structName) + ` struct \{\n((?:\t.*\n)*)\}`
				declaration := regexp.MustCompile(pattern).FindStringSubmatch(string(generated))
				require.Len(t, declaration, 2, "missing struct %s", structName)
				require.Regexp(t, `(?m)^\s+`+fieldName+`\s+`+regexp.QuoteMeta(typ)+"\\s+`json:", declaration[1])
			}
			if !testing.Short() {
				require.NoError(t, buildGoFile(test.name, generated))
			}
		})
	}
}

func TestNonNullSelfRecursiveInput(t *testing.T) {
	_, err := generateInputForTest(t,
		`input Input { next: Input! } type Query { test(input: Input): Boolean! }`,
		`query Recursive(
  $input: Input
) { test(input: $input) }`, &Config{})
	require.ErrorContains(t, err, "pointer on non-null input field can only be used together with omitempty: Input.next")
}

func TestSelfRecursiveInputJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping generated-code execution due to -short")
	}

	for _, omitEmpty := range []bool{false, true} {
		name, directive, leaf := "Null", "", `{"next":null}`
		if omitEmpty {
			name, directive, leaf = "OmitEmpty", "# @genqlient(for: \"Input.next\", omitempty: true)\n", `{}`
		}
		t.Run(name, func(t *testing.T) {
			generated, err := generateInputForTest(t,
				`input Input { next: Input } type Query { test(input: Input): Boolean! }`,
				directive+`query Recursive(
  $input: Input
) { test(input: $input) }`, &Config{})
			require.NoError(t, err)
			dir, err := os.MkdirTemp("testdata/tmp", "recursive_json_")
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, os.RemoveAll(dir)) })
			require.NoError(t, os.WriteFile(filepath.Join(dir, "generated.go"), generated, 0o600))
			// Exercise the actual generated type, not a handwritten equivalent.
			testCode := `package test
import (
  "encoding/json"
  "testing"
  "github.com/stretchr/testify/require"
)
func TestJSON(t *testing.T) {
  for _, tc := range []struct { value Input; json string } {
    {Input{}, ` + "`" + leaf + "`" + `},
    {Input{Next: &Input{}}, ` + "`" + `{"next":` + leaf + `}` + "`" + `},
    {Input{Next: &Input{Next: &Input{}}}, ` + "`" + `{"next":{"next":` + leaf + `}}` + "`" + `},
  } {
    data, err := json.Marshal(tc.value)
    require.NoError(t, err)
    require.JSONEq(t, tc.json, string(data))
    var decoded Input
    require.NoError(t, json.Unmarshal(data, &decoded))
    require.Equal(t, tc.value, decoded)
  }
  for _, data := range []string{"{}", "{\"next\":null}"} {
    var decoded Input
    require.NoError(t, json.Unmarshal([]byte(data), &decoded))
    require.Nil(t, decoded.Next)
  }
}`
			require.NoError(t, os.WriteFile(filepath.Join(dir, "generated_test.go"), []byte(testCode), 0o600))
			output, err := exec.Command("go", "test", "./"+filepath.ToSlash(dir)).CombinedOutput()
			require.NoError(t, err, "%s", output)
		})
	}
}

func generateInputForTest(t *testing.T, schema, operation string, config *Config) ([]byte, error) {
	t.Helper()
	dir := t.TempDir()
	config.Schema = []string{filepath.Join(dir, "schema.graphql")}
	config.Operations = []string{filepath.Join(dir, "query.graphql")}
	config.Generated = filepath.Join(dir, "generated.go")
	config.Package = "test"
	config.ContextType = "-"
	require.NoError(t, os.WriteFile(config.Schema[0], []byte(schema), 0o600))
	require.NoError(t, os.WriteFile(config.Operations[0], []byte(operation), 0o600))
	generated, err := Generate(config)
	return generated[config.Generated], err
}
