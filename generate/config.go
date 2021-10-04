package generate

import (
	"fmt"
	"go/token"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
	"gopkg.in/yaml.v2"
)

// Config represents genqlient's configuration, generally read from
// genqlient.yaml.
//
// Callers must call ValidateAndFillDefaults before using the config.
type Config struct {
	// The following fields are documented at:
	// https://github.com/Khan/genqlient/blob/main/docs/genqlient.yaml
	Schema           StringList              `yaml:"schema"`
	Operations       []string                `yaml:"operations"`
	Generated        string                  `yaml:"generated"`
	Package          string                  `yaml:"package"`
	ExportOperations string                  `yaml:"export_operations"`
	ContextType      string                  `yaml:"context_type"`
	ClientGetter     string                  `yaml:"client_getter"`
	Bindings         map[string]*TypeBinding `yaml:"bindings"`

	// Set to true to use features that aren't fully ready to use.
	//
	// This is primarily intended for genqlient's own tests.  These features
	// are likely BROKEN and come with NO EXPECTATION OF COMPATIBILITY.  Use
	// them at your own risk!
	AllowBrokenFeatures bool `yaml:"allow_broken_features"`

	// The directory of the config-file (relative to which all the other paths
	// are resolved).  Set by ValidateAndFillDefaults.
	baseDir string
}

// A TypeBinding represents a Go type to which genqlient will bind a particular
// GraphQL type, and is documented further at:
// https://github.com/Khan/genqlient/blob/main/docs/genqlient.yaml
type TypeBinding struct {
	Type              string `yaml:"type"`
	ExpectExactFields string `yaml:"expect_exact_fields"`
	Marshaler         string `yaml:"marshaler"`
	Unmarshaler       string `yaml:"unmarshaler"`
}

// ValidateAndFillDefaults ensures that the configuration is valid, and fills
// in any options that were unspecified.
//
// The argument is the directory relative to which paths will be interpreted,
// typically the directory of the config file.
func (c *Config) ValidateAndFillDefaults(baseDir string) error {
	c.baseDir = baseDir
	for i := range c.Schema {
		c.Schema[i] = filepath.Join(baseDir, c.Schema[i])
	}
	for i := range c.Operations {
		c.Operations[i] = filepath.Join(baseDir, c.Operations[i])
	}
	c.Generated = filepath.Join(baseDir, c.Generated)
	if c.ExportOperations != "" {
		c.ExportOperations = filepath.Join(baseDir, c.ExportOperations)
	}

	if c.ContextType == "" {
		c.ContextType = "context.Context"
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

// ReadAndValidateConfig reads the configuration from the given file, validates
// it, and returns it.
func ReadAndValidateConfig(filename string) (*Config, error) {
	text, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, errorf(nil, "unreadable config file %v: %v", filename, err)
	}

	var config Config
	err = yaml.UnmarshalStrict(text, &config)
	if err != nil {
		return nil, errorf(nil, "invalid config file %v: %v", filename, err)
	}

	err = config.ValidateAndFillDefaults(filepath.Dir(filename))
	if err != nil {
		return nil, errorf(nil, "invalid config file %v: %v", filename, err)
	}

	return &config, nil
}

func initConfig(filename string) error {
	// TODO(benkraft): Embed this config file into the binary, see
	// https://github.com/Khan/genqlient/issues/9.
	r, err := os.Open(filepath.Join(thisDir, "default_genqlient.yaml"))
	if err != nil {
		return err
	}
	w, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return errorf(nil, "unable to write default genqlient.yaml: %v", err)
	}
	_, err = io.Copy(w, r)
	return errorf(nil, "unable to write default genqlient.yaml: %v", err)
}

var path2regex = strings.NewReplacer(
	`.`, `\.`,
	`*`, `.+`,
	`\`, `[\\/]`,
	`/`, `[\\/]`,
)

// loadSchemaSources parses the schema file path globs. Parses graphql files,
// and returns the parsed ast.Source objects.
// Sourced From:
// 		https://github.com/99designs/gqlgen/blob/1a0b19feff6f02d2af6631c9d847bc243f8ede39/codegen/config/config.go#L129-L181
func loadSchemaSources(schemas StringList) ([]*ast.Source, error) {
	preGlobbing := schemas
	schemas = StringList{}
	source := make([]*ast.Source, 0)
	for _, f := range preGlobbing {
		var matches []string

		// for ** we want to override default globbing patterns and walk all
		// subdirectories to match schema files.
		if strings.Contains(f, "**") {
			pathParts := strings.SplitN(f, "**", 2)
			rest := strings.TrimPrefix(strings.TrimPrefix(pathParts[1], `\`), `/`)
			// turn the rest of the glob into a regex, anchored only at the end because ** allows
			// for any number of dirs in between and walk will let us match against the full path name
			globRe := regexp.MustCompile(path2regex.Replace(rest) + `$`)

			if err := filepath.Walk(pathParts[0], func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				if globRe.MatchString(strings.TrimPrefix(path, pathParts[0])) {
					matches = append(matches, path)
				}

				return nil
			}); err != nil {
				return nil, fmt.Errorf("failed to walk schema at root %s: %w", pathParts[0], err)
			}
		} else {
			var err error
			matches, err = filepath.Glob(f)
			if err != nil {
				return nil, fmt.Errorf("failed to glob schema filename %s: %w", f, err)
			}
		}

		for _, m := range matches {
			if schemas.Has(m) {
				continue
			}
			schemas = append(schemas, m)
		}
	}
	for _, filename := range schemas {
		filename = filepath.ToSlash(filename)
		var err error
		var schemaRaw []byte
		schemaRaw, err = ioutil.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("unable to open schema: %w", err)
		}

		source = append(source, &ast.Source{Name: filename, Input: string(schemaRaw)})
	}
	return source, nil
}
