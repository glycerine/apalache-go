package numeral

// Source: src/util/mpzzp.h:145-148; src/util/mpz.cpp:1432-1436.

import (
	"strconv"
	"strings"
)

// SourceStringTracked uses native Go string storage for the local stream and
// returned string. Only the actual project numeral-formatting scopes need an
// accepted receipt; no C++ standard-library object is represented.
func (m *IntegerModularManager) SourceStringTracked(a *Int, receipt *NumeralInsertionReceipt) (string, error) {
	if e := m.check(); e != nil {
		return "", e
	}
	if e := m.sourceAssertNormalized(a, 146); e != nil {
		return "", e
	}
	var text strings.Builder
	if e := m.manager.InsertSourceIntegerTracked(integerSourceStringStream{&text}, a, receipt, nil); e != nil {
		return "", e
	}
	return text.String(), nil
}

type integerSourceStringStream struct{ text *strings.Builder }

func (s integerSourceStringStream) AppendCString(v string) error { s.text.WriteString(v); return nil }
func (s integerSourceStringStream) AppendNumeralText(v string) error {
	s.text.WriteString(v)
	return nil
}
func (s integerSourceStringStream) AppendInt32(v int32) error {
	s.text.WriteString(strconv.FormatInt(int64(v), 10))
	return nil
}

// CleanupSourceString owns only the captured project-formatting children.
// An ended receipt can be retired without inspecting a later frame reuse.
func (m *IntegerModularManager) CleanupSourceString(receipt *NumeralInsertionReceipt) error {
	if e := m.check(); e != nil {
		return e
	}
	if receipt == nil || !receipt.Active() {
		return nil
	}
	return cleanupOwnedNumeralInsertion(receipt, m.manager.Naturals())
}
