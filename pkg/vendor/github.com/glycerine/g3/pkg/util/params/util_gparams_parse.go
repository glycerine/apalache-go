package params

import (
	"math"
	"strconv"
)

// Source public validation has already restricted the integer to digits.
// C strtol saturates its selected signed-long domain before unsigned narrowing.
func gparamsParseUint(s string, longBits uint8) uint32 {
	limit := uint64(1<<63 - 1)
	if longBits == 32 {
		limit = 1<<31 - 1
	}
	var n uint64
	for i := 0; i < len(s) && s[i] != 0; i++ {
		d := uint64(s[i] - '0')
		if n > (limit-d)/10 {
			n = limit
		} else {
			n = n*10 + d
		}
	}
	return uint32(n)
}

// Exact nearest-even overflow threshold: 2^1024 - 2^970. The selected C-locale
// IEEE64 adapter checks overflow BEFORE calling Go's parser, avoiding its
// otherwise escaping NumError/clone allocations. Tests derive this integer
// independently; no production big-number service or error string is used.
const gparamsFloatOverflowThreshold = "179769313486231580793728971405303415079934132710037826936173778980444968292764750946649017977587207096330286416692887910946555547851940402630657488671505820681908902000708383676273854845817711531764475730270069855571366959622842914819860834936475292719074168444365510704342711559699508093042880177904174497792"

// Source validation permits only digits, '.', '-', '/'. This scanner implements
// strtod's consumed decimal prefix in that domain; failed conversion yields +0.
// Other C locales, rounding modes, NaN/hex/exponent spellings are not reachable
// through actual public validation and are not claimed as a general C parser.
func gparamsParseDouble(s string) float64 {
	i := 0
	negative := false
	if len(s) > 0 && s[0] == '-' {
		negative = true
		i++
	}
	start := i
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	integerEnd := i
	digits := i - start
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			digits++
		}
	}
	if digits == 0 {
		return 0
	}
	for start < integerEnd && s[start] == '0' {
		start++
	}
	n := integerEnd - start
	overflow := n > len(gparamsFloatOverflowThreshold)
	if n == len(gparamsFloatOverflowThreshold) {
		// Comparison uses borrowed caller bytes; equality and any nonnegative
		// fractional suffix reach the same infinity endpoint.
		overflow = true
		for j := 0; j < n; j++ {
			if s[start+j] < gparamsFloatOverflowThreshold[j] {
				overflow = false
				break
			}
			if s[start+j] > gparamsFloatOverflowThreshold[j] {
				break
			}
		}
	}
	if overflow {
		if negative {
			return math.Inf(-1)
		}
		return math.Inf(1)
	}
	v, _ := strconv.ParseFloat(s[:i], 64)
	return v
}
