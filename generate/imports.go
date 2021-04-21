package generate

import (
	"fmt"
	"go/types"
	"regexp"
	"strconv"
	"strings"
)

func (g *generator) addImportFor(pkgPath string) (alias string) {
	if alias, ok := g.imports[pkgPath]; ok {
		return alias
	}

	pkgName := pkgPath[strings.LastIndex(pkgPath, "/")+1:]
	alias = pkgName
	suffix := 2
	for g.usedAliases[alias] {
		alias = pkgName + strconv.Itoa(suffix)
	}

	g.imports[pkgPath] = alias
	g.usedAliases[alias] = true
	return alias
}

// addRef adds any imports necessary to refer to the given name, and returns a
// reference alias.Name for it.
func (g *generator) addRef(fullyQualifiedName string) (qualifiedName string, err error) {
	return g.getRef(fullyQualifiedName, true)
}

// ref returns a reference alias.Name for the given import, if its package was
// already added (e.g. via addRef), and an error if not.
func (g *generator) ref(fullyQualifiedName string) (qualifiedName string, err error) {
	return g.getRef(fullyQualifiedName, false)
}

var _sliceOrMapPrefixRegexp = regexp.MustCompile(`^(\*|\[\d*\]|map\[string\])*`)

func (g *generator) getRef(fullyQualifiedName string, addImport bool) (qualifiedName string, err error) {
	// Ideally, we want to allow a reference to basically an arbitrary symbol.
	// But that's very hard, because it might be quite complicated, like
	//	struct{ F []map[mypkg.K]otherpkg.V }
	// Now in practice, using an unnamed struct is not a great idea, but we do
	// want to allow as much as we can that encoding/json knows how to work
	// with, since you would reasonably expect us to accept, say,
	// map[string][]interface{}.  So we allow:
	// - any named type (mypkg.T)
	// - any predeclared basic type (string, int, etc.)
	// - interface{}
	// - for any allowed type T, *T, []T, [N]T, and map[string]T
	// which effectively excludes:
	// - unnamed struct types
	// - map[K]V where K is a named type wrapping string
	// - any nonstandard spelling of those (interface {/* hi */},
	//	 map[  string      ]T)
	// TODO: document that somewhere visible

	errorMsg := `invalid type-name "%v" (%v); expected a builtin, ` +
		`path/to/package.Name, interface{}, or a slice, map, or pointer of those`

	if strings.Contains(fullyQualifiedName, " ") {
		// TODO: pass in pos here and below
		return "", errorf(nil, errorMsg, fullyQualifiedName, "contains spaces")
	}

	prefix := _sliceOrMapPrefixRegexp.FindString(fullyQualifiedName)
	nameToImport := fullyQualifiedName[len(prefix):]

	i := strings.LastIndex(nameToImport, ".")
	if i == -1 {
		if nameToImport != "interface{}" && types.Universe.Lookup(nameToImport) == nil {
			return "", errorf(nil, errorMsg, fullyQualifiedName,
				fmt.Sprintf(`unknown type-name "%v"`, nameToImport))
		}
		return fullyQualifiedName, nil
	}

	pkgPath := nameToImport[:i]
	localName := nameToImport[i+1:]
	var alias string
	if addImport {
		alias = g.addImportFor(pkgPath)
	} else {
		var ok bool
		alias, ok = g.imports[pkgPath]
		if !ok {
			// This is an internal error, not a user error.
			return "", errorf(nil, `no alias defined for package "%v"`, pkgPath)
		}
	}
	return prefix + alias + "." + localName, nil
}

// Returns the import-clause to use in the generated code.
func (g *generator) Imports() string {
	if len(g.imports) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("import (\n")
	for path, alias := range g.imports {
		if path == alias || strings.HasSuffix(path, "/"+alias) {
			builder.WriteString("\t" + strconv.Quote(path) + "\n")
		} else {
			builder.WriteString("\t" + alias + " " + strconv.Quote(path) + "\n")
		}
	}
	builder.WriteString(")\n\n")
	return builder.String()
}
