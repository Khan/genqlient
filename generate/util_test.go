package generate

import "testing"

type test struct {
	name string
	in   string
	out  string
}

func testStringFunc(t *testing.T, f func(string) string, tests []test) {
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			got := f(test.in)
			if got != test.out {
				t.Errorf("got %#v want %#v", got, test.out)
			}
		})
	}
}

func TestLowerFirst(t *testing.T) {
	tests := []test{
		{"Empty", "", ""},
		{"SingleLower", "l", "l"},
		{"SingleUpper", "L", "l"},
		{"SingleUnicodeLower", "ļ", "ļ"},
		{"SingleUnicodeUpper", "Ļ", "ļ"},
		{"LongerLower", "lasdf", "lasdf"},
		{"LongerUpper", "Lasdf", "lasdf"},
		{"LongerUnicodeLower", "ļasdf", "ļasdf"},
		{"LongerUnicodeUpper", "Ļasdf", "ļasdf"},
	}

	testStringFunc(t, lowerFirst, tests)
}

func TestUpperFirst(t *testing.T) {
	tests := []test{
		{"Empty", "", ""},
		{"SingleLower", "l", "L"},
		{"SingleUpper", "L", "L"},
		{"SingleUnicodeLower", "ļ", "Ļ"},
		{"SingleUnicodeUpper", "Ļ", "Ļ"},
		{"LongerLower", "lasdf", "Lasdf"},
		{"LongerUpper", "Lasdf", "Lasdf"},
		{"LongerUnicodeLower", "ļasdf", "Ļasdf"},
		{"LongerUnicodeUpper", "Ļasdf", "Ļasdf"},
	}

	testStringFunc(t, upperFirst, tests)
}

func TestMatchFirst(t *testing.T) {
	tests := []struct {
		name, in, out, match string
	}{
		{"Empty", "", "", ""},
		{"LowerToUpper", "lower", "Lower", "Upper"},
		{"UpperToUpper", "Upper", "Upper", "Upper"},
		{"LowerToLower", "lower", "lower", "lower"},
		{"UpperToLower", "Upper", "upper", "lower"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			got := matchFirst(test.in, test.match)
			if got != test.out {
				t.Errorf("got %#v want %#v", got, test.out)
			}
		})
	}
}

func TestGoConstName(t *testing.T) {
	tests := []test{
		{"Empty", "", ""},
		{"AllCaps", "ASDF", "Asdf"},
		{"AllCapsWithUnderscore", "ASDF_GH", "AsdfGh"},
		{"JustUnderscore", "_", "_"},
		{"LeadingUnderscore", "_ASDF_GH", "AsdfGh"},
	}

	testStringFunc(t, goConstName, tests)
}
