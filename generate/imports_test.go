package generate

import (
	"go/token"
	"testing"
)

func TestMakeIdentifier(t *testing.T) {
	tests := []struct {
		testName string
		input    string
		expected string
	}{
		{"GoodIdentifier", "myIdent", "myIdent"},
		{"GoodIdentifierNumbers", "myIdent1234", "myIdent1234"},
		{"NumberPrefix", "1234myIdent", "myIdent"},
		{"OnlyNumbers", "1234", "alias"},
		{"Dashes", "my-ident", "myident"},
		// Note: most Go implementations won't actually allow
		// this package-path, but the spec is pretty vague
		// so make sure to handle it.
		{"JunkAnd", "my!!\\\\\nident", "myident"},
		{"JunkOnly", "!!\\\\\n", "alias"},
		{"Accents", "née", "née"},
		{"Kanji", "日本", "日本"},
		{"EmojiAnd", "ident👍", "ident"},
		{"EmojiOnly", "👍", "alias"},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			actual := makeIdentifier(test.input)
			if actual != test.expected {
				t.Errorf("mismatch:\ngot:  %s\nwant: %s", actual, test.expected)
			}
			if !token.IsIdentifier(actual) {
				t.Errorf("not a valid identifier: %s", actual)
			}
		})
	}
}
