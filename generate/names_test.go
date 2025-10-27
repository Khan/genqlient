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
		// We don't shorten field-names.
		"OperationOperationUser",
		[]*ast.Field{fakeField("Query", "operationUser")},
		"User",
	}, {
		// We do shorten across multiple prefixes.
		"OperationUser",
		[]*ast.Field{fakeField("Query", "user")},
		"OperationUser",
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
		t.Run(test.expectedTypeName, func(t *testing.T) {
			prefix := newPrefixList("Operation")
			for _, field := range test.fields {
				prefix = nextPrefix(prefix, field, CasingDefault)
			}
			actualTypeName := makeTypeName(prefix, test.leafTypeName, CasingDefault)
			if actualTypeName != test.expectedTypeName {
				t.Errorf("name mismatch:\ngot:  %s\nwant: %s",
					actualTypeName, test.expectedTypeName)
			}
		})
	}
}

func TestSnakeToTypeNames(t *testing.T) {
	// Test specifically for the snake_case conversion in type names
	tests := []struct {
		expectedTypeName string
		fields           []*ast.Field
		leafTypeName     string
		autoCamelCase    bool
	}{{
		// Without auto_camel_case
		"ServiceIPsIp_address_listSnake_case_type",
		[]*ast.Field{fakeField("Query", "ip_address_list")},
		"snake_case_type",
		false,
	}, {
		// With auto_camel_case
		"ServiceIPsIpAddressListSnakeCaseType",
		[]*ast.Field{fakeField("Query", "ip_address_list")},
		"snake_case_type",
		true,
	}, {
		// With nested snake_case fields
		"ServiceIPsObjectSnake_case_fieldSnake_case_type",
		[]*ast.Field{fakeField("Query", "object"), fakeField("Object", "snake_case_field")},
		"snake_case_type",
		false,
	}, {
		// With nested snake_case fields and auto_camel_case enabled
		"ServiceIPsObjectSnakeCaseFieldSnakeCaseType",
		[]*ast.Field{fakeField("Query", "object"), fakeField("Object", "snake_case_field")},
		"snake_case_type",
		true,
	}}

	for _, test := range tests {
		t.Run(test.expectedTypeName, func(t *testing.T) {
			prefix := newPrefixList("ServiceIPs")
			algorithm := CasingDefault
			if test.autoCamelCase {
				algorithm = CasingAutoCamelCase
			}

			for _, field := range test.fields {
				prefix = nextPrefix(prefix, field, algorithm)
			}
			actualTypeName := makeTypeName(prefix, test.leafTypeName, algorithm)
			if actualTypeName != test.expectedTypeName {
				t.Errorf("name mismatch:\ngot:  %s\nwant: %s",
					actualTypeName, test.expectedTypeName)
			}
		})
	}
}

func TestTypeNameCollisions(t *testing.T) {
	tests := []struct {
		fields       []*ast.Field
		leafTypeName string
	}{
		{[]*ast.Field{fakeField("Query", "user")}, "UserInterface"},
		{[]*ast.Field{fakeField("Query", "user")}, "User"},
		{[]*ast.Field{fakeField("Query", "user")}, "QueryUser"},
		{[]*ast.Field{fakeField("Query", "queryUser")}, "User"},
		// Known issues, described in names.go file-documentation:
		// Interface/implementation collision:
		// 	{[]*ast.Field{fakeField("Query", "queryUser")}, "QueryUser"},
		// Case collision:
		// 	{[]*ast.Field{fakeField("Query", "QueryUser")}, "User"},
		// Overlapping-parts collision:
		//	{[]*ast.Field{fakeField("Query", "userQuery")}, "User"},
	}
	seen := map[string]int{} // name -> index of test that had it
	for i, test := range tests {
		prefix := newPrefixList("Operation")
		for _, field := range test.fields {
			prefix = nextPrefix(prefix, field, CasingDefault)
		}
		actualTypeName := makeTypeName(prefix, test.leafTypeName, CasingDefault)

		otherIndex, ok := seen[actualTypeName]
		if ok {
			t.Errorf("name collision:\ncase %2d: %#v\ncase %2d: %#v",
				i, test, otherIndex, tests[otherIndex])
		}
		seen[actualTypeName] = i
	}
}
