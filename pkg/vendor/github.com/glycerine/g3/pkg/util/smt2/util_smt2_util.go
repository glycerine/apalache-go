package smt2

// Source: src/util/smt2_util.h; src/util/smt2_util.cpp.

import (
	"errors"
	"strings"

	"github.com/glycerine/g3/pkg/arena"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
)

var ErrSMT2NeedsQuoting = errors.New("g3: SMT2 symbol does not satisfy quoting precondition")

func IsSMT2SimpleSymbolChar(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '~' || c == '!' || c == '@' || c == '$' || c == '%' || c == '^' || c == '&' || c == '*' || c == '_' || c == '-' || c == '+' || c == '=' || c == '<' || c == '>' || c == '.' || c == '?' || c == '/'
}

// IsSMT2QuotedCString is the source needs-quoting predicate, not a quoted parser.
// The input is borrowed only for this call; nil is the source null C pointer.
func IsSMT2QuotedCString(text *string) bool {
	if text == nil {
		return false
	}
	s := *text
	n := utext.CStringLength(s)
	s = s[:n]
	if n > 0 && s[0] >= '0' && s[0] <= '9' {
		return true
	}
	if n >= 2 && s[0] == '|' && s[n-1] == '|' {
		for i := 1; i+1 < n; i++ {
			if s[i] == '\\' && i+2 < n && (s[i+1] == '\\' || s[i+1] == '|') {
				i++
			} else if s[i] == '\\' || s[i] == '|' {
				return true
			}
		}
		return false
	}
	for i := 0; i < n; i++ {
		if !IsSMT2SimpleSymbolChar(s[i]) {
			return true
		}
	}
	return false
}
func smt2QuotedSymbol(m *usymbol.Manager, s usymbol.Symbol) (bool, error) {
	if e := m.Validate(s); e != nil {
		return false, e
	}
	if s.IsNumerical() || s.Value == 0 {
		return false, nil
	}
	text, _, e := m.BareString(s)
	if e != nil {
		return false, e
	}
	return IsSMT2QuotedCString(&text), nil
}
func IsSMT2QuotedSymbol(m *usymbol.Manager, s usymbol.Symbol) (bool, error) {
	if m == nil {
		return false, usymbol.ErrInvalid
	}
	return smt2QuotedSymbol(m, s)
}

// MkSMT2QuotedSymbol reads an immutable spelling from its checked owner and
// returns the quoted Go string. Temporary formatting storage is Go-managed.
func MkSMT2QuotedSymbol(m *usymbol.Manager, s usymbol.Symbol, _ *arena.Budget) (string, error) {
	if m == nil {
		return "", usymbol.ErrInvalid
	}
	needed, e := smt2QuotedSymbol(m, s)
	if e != nil {
		return "", e
	}
	if !needed {
		return "", ErrSMT2NeedsQuoting
	}
	var b strings.Builder
	b.WriteByte('|')
	text, _, e := m.BareString(s)
	if e != nil {
		return "", e
	}
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c == '|' || c == '\\' {
			b.WriteByte('\\')
		}
		b.WriteByte(c)
	}
	b.WriteByte('|')
	return b.String(), nil
}
