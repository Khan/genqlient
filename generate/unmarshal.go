package generate

var unmarshalTemplate = mustTemplate("unmarshal.go.tmpl")

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

func (builder *typeBuilder) maybeWriteUnmarshal(fields []field) error {
	data := templateData{Type: builder.typeName}
	for _, field := range fields {
		typedef := builder.schema.Types[field.Type().Name()]
		if typedef.IsAbstractType() {
			fieldInfo := abstractField{
				GoName:   upperFirst(field.Alias()),
				JSONName: field.Alias(),
			}
			for _, typedef := range builder.schema.GetPossibleTypes(typedef) {
				fieldInfo.ConcreteTypes = append(fieldInfo.ConcreteTypes,
					concreteType{
						// TODO: this is quite fragile (and wrong if the
						// field name + type name are the same)
						GoName:      builder.typeNamePrefix + fieldInfo.GoName + upperFirst(typedef.Name),
						GraphQLName: typedef.Name,
					})
			}
			data.Fields = append(data.Fields, fieldInfo)
		}
	}

	if len(data.Fields) == 0 {
		return nil
	}

	builder.WriteString("\n\n")
	return unmarshalTemplate.Execute(builder, data)
}
