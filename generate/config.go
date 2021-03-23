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
	Queries:     "queries.graphql",
	Generated:   "generated.go",
	ContextType: "context.Context",
}

type Config struct {
	// The package name for the output code; defaults to the directory name of
	// Generated
	Package string `yaml:"package"`
	// The filename with the GraphQL schema (in SDL format); defaults to
	// schema.graphql
	// TODO: Allow fetching a schema via introspection (will need to figure out
	// how to convert that to SDL).
	Schema string `yaml:"schema"`
	// The filename with the queries; defaults to queries.graphql
	Queries string `yaml:"queries"`
	// The filename to which to write the generated code; defaults to
	// generated.go
	Generated string `yaml:"generated"`
	// Set to the fully-qualified name of a type which generated helpers should
	// accept and use as the context.Context for HTTP requests.  Defaults to
	// context.Context; set to the empty string to omit context entirely.
	ContextType string `yaml:"context_type"`
	// TODO: implement client-getters
	// If set, a snippet of Go code to get a *graphql.Client from the context
	// (which will be named ctx).  For example, this might do
	// ctx.Value(myKey).(*graphql.Client).  If omitted, client must be
	// passed to each method explicitly.
	// TODO: what if you want to do an import in this snippet, e.g. for a
	// getter function, global var, or a context-key-type?
	// TODO: what if you want to return err?
	// ClientGetter string `yaml:"client_getter"`
}

func (c *Config) ValidateAndFillDefaults() error {
	if c.Package == "" {
		abs, err := filepath.Abs(c.Generated)
		if err != nil {
			return fmt.Errorf("unable to guess package-name: %v", err)
		}

		base := filepath.Base(abs)
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

	err := config.ValidateAndFillDefaults()
	if err != nil {
		return nil, fmt.Errorf("invalid config file %v: %v", filename, err)
	}

	// Make paths relative to config dir
	basename := filepath.Dir(filename)
	config.Schema = filepath.Join(basename, config.Schema)
	config.Queries = filepath.Join(basename, config.Queries)
	config.Generated = filepath.Join(basename, config.Generated)

	return &config, nil
}
