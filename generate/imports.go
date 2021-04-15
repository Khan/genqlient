package generate

import (
	"fmt"
	"go/types"
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

func (g *generator) getRef(fullyQualifiedName string, addImport bool) (qualifiedName string, err error) {
	i := strings.LastIndex(fullyQualifiedName, ".")
	if i == -1 {
		// We allow any builtin type, or interface{}.  In principle it would be
		// fine to allow any interface or struct, but (1) they might refer to a
		// type that needs an import, and (2) that just honestly seems
		// confusing, why would you want it.  But the empty interface,
		// specifically, is useful.
		if fullyQualifiedName != "interface{}" && types.Universe.Lookup(fullyQualifiedName) == nil {
			return "", fmt.Errorf(
				`unknown name "%v"; expected a builtin or path/to/package.Name`, fullyQualifiedName)
		}
		return fullyQualifiedName, nil
	}

	pkgPath := fullyQualifiedName[:i]
	localName := fullyQualifiedName[i+1:]
	var alias string
	if addImport {
		alias = g.addImportFor(pkgPath)
	} else {
		var ok bool
		alias, ok = g.imports[pkgPath]
		if !ok {
			return "", fmt.Errorf(`no alias defined for package "%v"`, pkgPath)
		}
	}
	return alias + "." + localName, nil
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
