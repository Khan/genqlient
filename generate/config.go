package generate

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

var defaultConfig = &Config{
	Schema:    "schema.graphql",
	Queries:   "queries.graphql",
	Generated: "generated.go",
}

type Config struct {
	// The package name for the output code; defaults to the directory name of
	// Generated
	Package string `yaml:"package"`
	// The filename with the GraphQL schema (in SDL format); defaults to
	// schema.graphql
	// TODO: allow other formats
	// TODO: allow URLs
	Schema string `yaml:"schema"`
	// The filename with the queries; defaults to queries.graphql
	Queries string `yaml:"queries"`
	// The filename to which to write the generated code; defaults to
	// generated.go
	Generated string `yaml:"generated"`
}

func (c *Config) ValidateAndFillDefaults() error {
	if c.Schema == "" {
		c.Schema = defaultConfig.Schema
	}
	if c.Queries == "" {
		c.Queries = defaultConfig.Queries
	}
	if c.Generated == "" {
		c.Generated = defaultConfig.Generated
	}

	if c.Package == "" {
		abs, err := filepath.Abs(c.Generated)
		if err != nil {
			return fmt.Errorf("unable to guess package-name: %v", err)
		}

		base := filepath.Base(abs)
		// TODO: remove/replace bad chars, make sure there's something left?
		c.Package = base
	}

	return nil
}

func ReadAndValidateConfig(filename string) (*Config, error) {
	var config Config
	if filename == "" {
		config = *defaultConfig
	} else {
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
	// TODO: more principled typing here?
	basename := filepath.Dir(filename)
	config.Schema = filepath.Join(basename, config.Schema)
	config.Queries = filepath.Join(basename, config.Queries)
	config.Generated = filepath.Join(basename, config.Generated)

	return &config, nil
}
