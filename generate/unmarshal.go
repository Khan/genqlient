package generate

import "io"

// TODO(benkraft): We could potentially get rid of these now, and do everything
// directly from the types.
type templateData struct {
	// Go type to which the method will be added
	Type string
	// Abstract fields of the type, which need special handling.
	Fields []abstractField
}

type abstractField struct {
	// Name of the field, in Go and JSON
	GoName, JSONName string
	// Concrete types the field might take.
	ConcreteTypes []concreteType
}

type concreteType struct {
	// Name of the type, in Go and GraphQL
	GoName, GraphQLName string
}

func (g *generator) maybeWriteUnmarshal(w io.Writer, typ *goStructType) error {
	data := templateData{Type: typ.GoName}
	for _, field := range typ.Fields {
		// TODO(benkraft): To handle list-of-interface fields, we should really
		// be "unwrapping" any goSliceType/goPointerType wrappers to find the
		// goInterfaceType.
		if iface, ok := field.GoType.(*goInterfaceType); ok {
			fieldInfo := abstractField{
				GoName:   field.GoName,
				JSONName: field.JSONName,
			}
			for _, impl := range iface.Implementations {
				fieldInfo.ConcreteTypes = append(fieldInfo.ConcreteTypes,
					concreteType{
						GoName:      impl.GoName,
						GraphQLName: impl.GraphQLName,
					})
			}
			data.Fields = append(data.Fields, fieldInfo)
		}
	}

	if len(data.Fields) == 0 {
		return nil
	}

	_, err := g.addRef("encoding/json.Unmarshal")
	if err != nil {
		return err
	}

	return g.execute("unmarshal.go.tmpl", w, data)
}
