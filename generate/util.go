package generate

import (
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
