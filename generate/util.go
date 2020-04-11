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
	return changeFirst(s, unicode.ToLower)
}

func upperFirst(s string) string {
	return changeFirst(s, unicode.ToUpper)
}

func goConstName(s string) string {
	var prev rune
	return strings.Map(func(r rune) rune {
		var ret rune
		if prev == 0 && r == '_' {
			return '_' // still treat next char as first
		} else if r == '_' {
			ret = -1
		} else if prev == '_' {
			ret = unicode.ToUpper(r)
		} else {
			ret = unicode.ToLower(r)
		}
		prev = r
		return ret
	}, s)
}
