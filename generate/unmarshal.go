package generate

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

func (builder *typeBuilder) maybeWriteUnmarshal(typeName, typeNamePrefix string, fields []field) error {
	data := templateData{Type: typeName}
	for _, field := range fields {
		typedef := builder.schema.Types[field.Type().Name()]
		if typedef.IsAbstractType() {
			fieldInfo := abstractField{
				GoName:   upperFirst(field.Alias()),
				JSONName: field.Alias(),
			}
			for _, typedef := range builder.schema.GetPossibleTypes(typedef) {
				// TODO: this is slightly fragile (it needs to match the
				// similar call in writeField)
				goName, _ := builder.typeName(typeNamePrefix+fieldInfo.GoName, typedef)
				fieldInfo.ConcreteTypes = append(fieldInfo.ConcreteTypes,
					concreteType{
						GoName:      goName,
						GraphQLName: typedef.Name,
					})
			}
			data.Fields = append(data.Fields, fieldInfo)
		}
	}

	if len(data.Fields) == 0 {
		return nil
	}

	_, err := builder.addRef("encoding/json.Unmarshal")
	if err != nil {
		return err
	}

	builder.WriteString("\n\n")
	return builder.execute("unmarshal.go.tmpl", builder, data)
}
