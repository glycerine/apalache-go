package numeral

// Source: src/util/mpbq.cpp:384-392; src/util/mpz.cpp:1432-1436.

import (
	"strconv"
	"strings"
)

// SourceStringTracked keeps the actual numeral-formatting child receipt.
// Native Go storage replaces ostringstream and std::string. The source reads
// the denominator power only after the numerator string has completed.
func (m *BinaryRationalManager) SourceStringTracked(a *BinaryRational, r *NumeralInsertionReceipt) (string, error) {
	if e := m.check(a); e != nil {
		return "", e
	}
	var text strings.Builder
	if e := m.Z.InsertSourceIntegerTracked(integerSourceStringStream{&text}, a.BorrowNumerator(), r, nil); e != nil {
		return "", e
	}
	if a.KField == 1 {
		text.WriteString("/2")
	} else if a.KField > 1 {
		text.WriteString("/2^")
		text.WriteString(strconv.FormatUint(uint64(a.KField), 10))
	}
	return text.String(), nil
}

// CleanupSourceString owns only the captured project formatting children.
// It does not acquire unrelated binary-manager work or a later formatter reuse.
func (m *BinaryRationalManager) CleanupSourceString(r *NumeralInsertionReceipt) error {
	if m == nil || m.self != m || m.Z == nil {
		return ErrBinaryOwner
	}
	if r == nil || !r.Active() {
		return nil
	}
	return cleanupOwnedNumeralInsertion(r, m.Z.Naturals())
}
