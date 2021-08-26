package generate

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

func reverse(slice []string) {
	for left, right := 0, len(slice)-1; left < right; left, right = left+1, right-1 {
		slice[left], slice[right] = slice[right], slice[left]
	}
}

func changeFirst(s string, f func(rune) rune) string {
	c, n := utf8.DecodeRuneInString(s)
	if c == utf8.RuneError { // empty or invalid
		return s
	}
	return string(f(c)) + s[n:]
}

func lowerFirst(s string) string {
	return changeFirst(strings.TrimLeft(s, "_"), unicode.ToLower)
}

func upperFirst(s string) string {
	return changeFirst(strings.TrimLeft(s, "_"), unicode.ToUpper)
}

func goConstName(s string) string {
	if strings.TrimLeft(s, "_") == "" {
		return s
	}
	var prev rune
	return strings.Map(func(r rune) rune {
		var ret rune
		if r == '_' {
			ret = -1
		} else if prev == '_' || prev == 0 {
			ret = unicode.ToUpper(r)
		} else {
			ret = unicode.ToLower(r)
		}
		prev = r
		return ret
	}, s)
}
