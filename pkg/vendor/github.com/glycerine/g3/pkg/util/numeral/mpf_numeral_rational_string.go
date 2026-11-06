package numeral

// Source: src/util/f2n.h:164, instantiated as f2n<mpf_manager>.

import (
	"io"

	utext "github.com/glycerine/g3/pkg/util/text"
)

func (m *ArbitraryFloatNumeralManager) ToRationalString(x *ArbitraryFloat) (string, error) {
	if e := m.guard(); e != nil {
		return "", e
	}
	var owned utext.OwnedString
	// A pre-existing stopped conversion belongs to the core's caller. Only
	// finish the lexical scope created by this invocation on failure.
	previousPending := m.core.rationalString.active
	if e := m.core.ToRationalString(x, &owned); e != nil {
		if !previousPending {
			e = JoinNumericErrors(e, m.core.CleanupRationalString())
		}
		owned.SourceDestroy()
		return "", e
	}
	defer owned.SourceDestroy()
	return owned.View().String()
}

// ToString materializes the source's borrowed c_str while its concrete text
// context remains live, then closes that context before returning Go text.
func (m *ArbitraryFloatNumeralManager) ToString(x *ArbitraryFloat) (result string, cause error) {
	if e := m.guard(); e != nil {
		return "", e
	}
	owned, e := NewArbitraryFloatFormatText(m.core.budget)
	if e != nil {
		return "", e
	}
	defer func() { cause = JoinNumericErrors(cause, owned.Close()) }()
	view, e := m.core.ToStringInto(x, owned)
	if e != nil {
		return "", e
	}
	n, e := view.Length()
	if e != nil {
		return "", e
	}
	data := make([]byte, n)
	for i := range data {
		data[i], e = view.Byte(uint64(i))
		if e != nil {
			return "", e
		}
	}
	return string(data), nil
}

func (m *ArbitraryFloatNumeralManager) DisplayTo(w io.Writer, x *ArbitraryFloat) error {
	if w == nil {
		return ErrNumericOwner
	}
	s, e := m.ToString(x)
	if e != nil {
		return e
	}
	n, e := io.WriteString(w, s)
	if e != nil {
		return e
	}
	if n != len(s) {
		return io.ErrShortWrite
	}
	return nil
}
