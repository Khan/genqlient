package generate

import (
	"testing"

	"github.com/vektah/gqlparser/v2/ast"
)

func fakeField(containingTypeName, fieldName string) *ast.Field {
	// (just the fields we need, probably not usable outside this file)
	return &ast.Field{
		Alias:            fieldName,
		ObjectDefinition: &ast.Definition{Name: containingTypeName},
	}
}

func TestTypeNames(t *testing.T) {
	tests := []struct {
		expectedTypeName string
		fields           []*ast.Field
		leafTypeName     string
	}{{
		"OperationFieldType",
		[]*ast.Field{fakeField("Query", "field")},
		"Type",
	}, {
		"OperationUser",
		[]*ast.Field{fakeField("Query", "user")},
		"User",
	}, {
		"OperationFavoriteUser",
		[]*ast.Field{fakeField("Query", "favoriteUser")},
		"User",
	}, {
		"OperationField1Type1Field2Type2",
		[]*ast.Field{fakeField("Query", "field1"), fakeField("Type1", "field2")},
		"Type2",
	}, {
		"OperationUpperFieldLowerType",
		// This is legal GraphQL!
		[]*ast.Field{fakeField("Query", "UpperField")},
		"lowerType",
	}, {
		"OperationUpperLowerUpperLower",
		[]*ast.Field{fakeField("Query", "Upper"), fakeField("lower", "Upper")},
		"lower",
	}}
	for _, test := range tests {
		test := test
		t.Run(test.expectedTypeName, func(t *testing.T) {
			prefix := &prefixList{last: "Operation"}
			for _, field := range test.fields {
				prefix = nextPrefix(prefix, field)
			}
			actualTypeName := makeTypeName(prefix, test.leafTypeName)
			if actualTypeName != test.expectedTypeName {
				t.Errorf("name mismatch:\ngot:  %s\nwant: %s",
					actualTypeName, test.expectedTypeName)
			}
		})
	}
}
