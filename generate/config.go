package generate

import (
	"fmt"
	"go/token"
	"io/ioutil"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

var defaultConfig = &Config{
	Schema:      "schema.graphql",
	Queries:     []string{"queries.graphql"},
	Generated:   "generated.go",
	ContextType: "context.Context",
}

type Config struct {
	// The filename with the GraphQL schema (in SDL format); defaults to
	// schema.graphql
	// TODO: Allow fetching a schema via introspection (will need to figure out
	// how to convert that to SDL).
	Schema string `yaml:"schema"`

	// Filenames or globs with the queries; defaults to queries.graphql.
	//
	// These may be .graphql files, containing the queries in SDL format, or
	// Go files, in which case any string-literal starting with (optional
	// whitespace and) the string "# @genqlient" will be extracted as a query.
	Queries []string `yaml:"queries"`

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
	// TODO: what if you want to do an import in this snippet, e.g. for a
	// getter function, global var, or a context-key-type?
	// TODO: what if you want to return err?
	ClientGetter string `yaml:"client_getter"`
}

func (c *Config) ValidateAndFillDefaults(configFilename string) error {
	// Make paths relative to config dir
	configDir := filepath.Dir(configFilename)
	c.Schema = filepath.Join(configDir, c.Schema)
	for i := range c.Queries {
		c.Queries[i] = filepath.Join(configDir, c.Queries[i])
	}
	c.Generated = filepath.Join(configDir, c.Generated)

	if c.Package == "" {
		abs, err := filepath.Abs(c.Generated)
		if err != nil {
			return fmt.Errorf("unable to guess package-name: %v", err)
		}

		base := filepath.Base(filepath.Dir(abs))
		if !token.IsIdentifier(base) {
			return fmt.Errorf("unable to guess package-name: %v is not a valid identifier", base)
		}

		c.Package = base
	}

	return nil
}

func (c *Config) ContextPackage() string {
	if c.ContextType == "" {
		return ""
	}

	i := strings.LastIndex(c.ContextType, ".")
	return c.ContextType[:i]
}

func ReadAndValidateConfig(filename string) (*Config, error) {
	config := *defaultConfig
	if filename != "" {
		text, err := ioutil.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("unreadable config file %v: %v", filename, err)
		}

		err = yaml.Unmarshal(text, &config)
		if err != nil {
			return nil, fmt.Errorf("invalid config file %v: %v", filename, err)
		}
	}

	err := config.ValidateAndFillDefaults(filename)
	if err != nil {
		return nil, fmt.Errorf("invalid config file %v: %v", filename, err)
	}

	return &config, nil
}
