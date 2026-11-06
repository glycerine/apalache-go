package shell

// Source: src/shell/main.cpp:207-214, the -T option branch.

import (
	"errors"
	"math"
	"strconv"

	"github.com/glycerine/g3/pkg/util/text"
)

var ErrMissingGlobalTimeout = errors.New("option argument (-T:timeout) is missing.")
var ErrGlobalTimeoutRange = errors.New("global timeout seconds exceed the defined signed-long multiplication range")

// ApplyGlobalTimeoutOption consumes the colon argument after shell/main has
// selected -T. The source reads a signed decimal prefix with strtol, multiplies
// by 1000, then passes that long to set_timeout. Go's strconv handles the
// decimal conversion; overflowing source multiplication is an undefined C++
// domain and is rejected here before it can silently wrap.
func ApplyGlobalTimeoutOption(argument *string, timer *TimeoutService) error {
	if argument == nil {
		return ErrMissingGlobalTimeout
	}
	s := (*argument)[:text.CStringLength(*argument)]
	i := 0
	for i < len(s) {
		if s[i] != ' ' && s[i] != '\t' && s[i] != '\n' && s[i] != '\v' && s[i] != '\f' && s[i] != '\r' {
			break
		}
		i++
	}
	start := i
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	digits := i
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	var seconds int64
	if i != digits {
		parsed, err := strconv.ParseInt(s[start:i], 10, 64)
		if err != nil {
			if s[start] == '-' {
				seconds = math.MinInt64
			} else {
				seconds = math.MaxInt64
			}
		} else {
			seconds = parsed
		}
	}
	if seconds > math.MaxInt64/1000 || seconds < math.MinInt64/1000 {
		return ErrGlobalTimeoutRange
	}
	return timer.SetTimeout(seconds * 1000)
}
