package text

// Source: src/util/symbol.cpp; src/util/params.cpp.

import "strings"

func CStringLength(text string) int {
	if i := strings.IndexByte(text, 0); i >= 0 {
		return i
	}
	return len(text)
}
