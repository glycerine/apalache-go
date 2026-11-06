package api

import "strings"

func z3LogCString(value string) string {
	if nul := strings.IndexByte(value, 0); nul >= 0 {
		return value[:nul]
	}
	return value
}

// z3LogEscape is ll_escaped from z3_logger.h. Escaped bytes use three
// decimal digits because the replay scanner deliberately does the same.
func z3LogEscape(value string) string {
	value = z3LogCString(value)
	var result strings.Builder
	result.Grow(len(value))
	for i := 0; i < len(value); i++ {
		c := value[i]
		if z3LogPlain(c) {
			result.WriteByte(c)
			continue
		}
		result.WriteByte('\\')
		result.WriteByte('0' + c/100)
		result.WriteByte('0' + (c/10)%10)
		result.WriteByte('0' + c%10)
	}
	return result.String()
}

func z3LogPlain(c byte) bool {
	if c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
		return true
	}
	return strings.ContainsRune("~!@#$%^&*-_+.?/ <>", rune(c))
}
