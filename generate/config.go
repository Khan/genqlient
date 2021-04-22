package generate

import (
	"go/token"
	"io/ioutil"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

var defaultConfig = &Config{
	Schema:      "schema.graphql",
	Operations:  []string{"genqlient.graphql"},
	Generated:   "generated.go",
	ContextType: "context.Context",
}

type Config struct {
	// The filename with the GraphQL schema (in SDL format); defaults to
	// schema.graphql
	Schema string `yaml:"schema"`

	// Filenames or globs with the operations for which to generate code;
	// defaults to genqlient.graphql.
	//
	// These may be .graphql files, containing the queries in SDL format, or
	// Go files, in which case any string-literal starting with (optional
	// whitespace and) the string "# @genqlient" will be extracted as a query.
	Operations []string `yaml:"operations"`

	// If set, a file at this path will be generated containing the exact
	// operations that genqlient will send to the server.
	//
	// This is useful for systems which require queries to be explicitly
	// safelisted, especially for cases like queries involving fragments where
	// it may not exactly match the input queries.  The JSON is an object of
	// the form
	//	{"operations": [{
	//		"operationName": "operationname",
	//		"query": "query operationName { ... }",
	//		"sourceLocation": "myqueriesfile.graphql",
	//	}]}
	// Keys may be added in the future.
	//
	// By default, no such file is written.
	ExportOperations string `yaml:"export_operations"`

	// The filename to which to write the generated code; defaults to
	// generated.go
	Generated string `yaml:"generated"`

	// The package name for the output code; defaults to the directory name of
	// Generated
	Package string `yaml:"package"`

	// Set to the fully-qualified name of a type which generated helpers should
	// accept and use as the context.Context for HTTP requests.  Defaults to
	// context.Context; set to the empty string to omit context entirely.
	ContextType string `yaml:"context_type"`

	// If set, a snippet of Go code to get a *graphql.Client from the context
	// (which will be named ctx).  For example, this might do
	// ctx.Value(myKey).(*graphql.Client).  If omitted, client must be
	// passed to each method explicitly.
	// TODO(#5): This is a bit broken, fix it.
	ClientGetter string `yaml:"client_getter"`

	// A map from GraphQL scalar type name to Go fully-qualified type name for
	// the types to use for any custom or builtin scalars.  By default, builtin
	// scalars are mapped to the obvious Go types (String and ID to string, Int
	// to int, Float to float64, and Boolean to bool), but this setting will
	// extend or override those mappings.  These types must define MarshalJSON
	// and UnmarshalJSON methods, or otherwise be convertible to JSON.
	Scalars map[string]string `yaml:"scalars"`

	// Set automatically to the filename of the config file itself.
	configFilename string
}

// baseDir returns the directory of the config-file (relative to which
// all the other paths are resolved).
func (c *Config) baseDir() string {
	return filepath.Dir(c.configFilename)
}

func (c *Config) ValidateAndFillDefaults(configFilename string) error {
	c.configFilename = configFilename
	// Make paths relative to config dir
	c.Schema = filepath.Join(c.baseDir(), c.Schema)
	for i := range c.Operations {
		c.Operations[i] = filepath.Join(c.baseDir(), c.Operations[i])
	}
	c.Generated = filepath.Join(c.baseDir(), c.Generated)
	if c.ExportOperations != "" {
		c.ExportOperations = filepath.Join(c.baseDir(), c.ExportOperations)
	}

	if c.Package == "" {
		abs, err := filepath.Abs(c.Generated)
		if err != nil {
			return errorf(nil, "unable to guess package-name: %v", err)
		}

		base := filepath.Base(filepath.Dir(abs))
		if !token.IsIdentifier(base) {
			return errorf(nil, "unable to guess package-name: %v is not a valid identifier", base)
		}

		c.Package = base
	}

	return nil
}

func ReadAndValidateConfig(filename string) (*Config, error) {
	config := *defaultConfig
	if filename != "" {
		text, err := ioutil.ReadFile(filename)
		if err != nil {
			return nil, errorf(nil, "unreadable config file %v: %v", filename, err)
		}

		err = yaml.Unmarshal(text, &config)
		if err != nil {
			return nil, errorf(nil, "invalid config file %v: %v", filename, err)
		}
	}

	err := config.ValidateAndFillDefaults(filename)
	if err != nil {
		return nil, errorf(nil, "invalid config file %v: %v", filename, err)
	}

	return &config, nil
}
